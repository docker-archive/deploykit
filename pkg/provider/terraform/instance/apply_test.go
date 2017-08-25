package instance

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
)

func TestRunTerraformApply(t *testing.T) {
	// Run this test locally only if terraform is set up
	if SkipTests("terraform") {
		t.SkipNow()
	}
	dir, err := os.Getwd()
	require.NoError(t, err)
	dir = path.Join(dir, "aws-two-tier")
	terraform := NewTerraformInstancePlugin(dir, 1*time.Second, false, nil)
	p, _ := terraform.(*plugin)
	err = p.doTerraformApply()
	require.NoError(t, err)
}

func TestContinuePollingStandalone(t *testing.T) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	terraform := NewTerraformInstancePlugin(dir, 1*time.Second, true, nil)
	p, _ := terraform.(*plugin)
	shoudApply := p.shouldApply()
	require.True(t, shoudApply)
}

// resInfo holds the resource type and resource name
type resInfo struct {
	ResType  TResourceType
	ResName  TResourceName
	ResProps TResourceProperties
}

// fileInfo holds the data for a file to create in the plugin's working dir
type fileInfo struct {
	ResInfo    []resInfo
	NewFile    bool
	FilePrefix string
	Plugin     *plugin
}

// writeFile is a utility function to write out a terraform file
func writeFile(info fileInfo, t *testing.T) {
	require.NotZero(t, len(info.ResInfo))
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	for _, resInfo := range info.ResInfo {
		if len(resInfo.ResProps) == 0 {
			resInfo.ResProps = TResourceProperties{"key": "val"}
		}
		inst[resInfo.ResType] = map[TResourceName]TResourceProperties{
			resInfo.ResName: resInfo.ResProps,
		}
	}
	buff, err := json.MarshalIndent(TFormat{Resource: inst}, " ", " ")
	require.NoError(t, err)
	// Default file prefix to the name of the first instance
	if info.FilePrefix == "" {
		info.FilePrefix = string(info.ResInfo[0].ResName)
	}
	var filename string
	if info.NewFile {
		filename = fmt.Sprintf("%v.tf.json.new", info.FilePrefix)
	} else {
		filename = fmt.Sprintf("%v.tf.json", info.FilePrefix)
	}
	err = afero.WriteFile(info.Plugin.fs, filepath.Join(info.Plugin.Dir, filename), buff, 0644)
	require.NoError(t, err)
}

// getFilesname is a utility function to a list of terraform files
func getFilenames(t *testing.T, p *plugin) (tfFiles []string, tfNewFiles []string) {
	fs := &afero.Afero{Fs: p.fs}
	err := fs.Walk(p.Dir,
		func(path string, info os.FileInfo, err error) error {
			if strings.HasSuffix(info.Name(), ".tf.json") {
				tfFiles = append(tfFiles, info.Name())
			} else if strings.HasSuffix(info.Name(), ".tf.json.new") {
				tfNewFiles = append(tfNewFiles, info.Name())
			}
			return nil
		},
	)
	require.NoError(t, err)
	return
}

func TestHandleFilesRefreshFail(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 10 files, should not be changed since refresh failed
	for i := 100; i < 110; i++ {
		newFile := false
		if i >= 105 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}

	fns := tfFuncs{
		tfRefresh: func() error {
			return fmt.Errorf("Custom refresh error")
		},
	}
	err := tf.handleFiles(fns)
	require.Error(t, err)
	require.Equal(t, "Custom refresh error", err.Error())

	// No files changed
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 5)
	require.Len(t, tfFiles, 5)
	for i := 100; i < 105; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
	for i := 105; i < 110; i++ {
		require.Contains(t, tfFilesNew, fmt.Sprintf("instance-%v.tf.json.new", i))
	}
}

func TestHandleFilesStateListFail(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	refreshInvoked := false
	fns := tfFuncs{
		tfRefresh: func() error {
			refreshInvoked = true
			return nil
		},
		tfStateList: func(dirArg string) (map[TResourceType]map[TResourceName]struct{}, error) {
			require.Equal(t, dir, dirArg)
			return nil, fmt.Errorf("Custom state list error")
		},
	}
	err := tf.handleFiles(fns)
	require.Error(t, err)
	require.Equal(t, "Custom state list error", err.Error())
	require.True(t, refreshInvoked)
}

func TestHandleFilesNoFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			return map[TResourceType]map[TResourceName]struct{}{}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)
}

func TestHandleFilesNoPruneNoNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 5 files
	for i := 100; i < 105; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 5)

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return 5 files
			result := map[TResourceName]struct{}{}
			for i := 100; i < 105; i++ {
				result[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
			}
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: result,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// No files removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 5)
	for i := 100; i < 105; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilesDedicatedGlobalNoPruneNoNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 5 files with VMs, 5 with VMs and a default, 5 with VMs
	// and dedicated, and a global
	for i := 100; i < 105; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 105; i < 110; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
				{
					ResType: TResourceType("default-nfs"),
					ResName: TResourceName(fmt.Sprintf("default-nfs-instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 110; i < 115; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
		// Dedicated
		info = fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("dedicated-nfs"),
					ResName: TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)),
				},
			},
			FilePrefix: fmt.Sprintf("dedicated-nfs-instance-%v", i),
			NewFile:    false,
			Plugin:     tf,
		}
		writeFile(info, t)
	}
	for i := 115; i < 117; i++ {
		// Global
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("global-nfs"),
					ResName: TResourceName(fmt.Sprintf("global-nfs-instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	// 15 VMs, 5 dedicated NFS, 2 global NFS
	require.Len(t, tfFiles, 15+5+2)
	for i := 100; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
	for i := 110; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
	}
	for i := 115; i < 117; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
	}

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return all VMs
			vms := map[TResourceName]struct{}{}
			for i := 100; i < 115; i++ {
				vms[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
			}
			// The default NFS
			defaultNFS := map[TResourceName]struct{}{}
			for i := 105; i < 110; i++ {
				defaultNFS[TResourceName(fmt.Sprintf("default-nfs-instance-%v", i))] = struct{}{}
			}
			// The dedicated NFS
			dedicatedNFS := map[TResourceName]struct{}{}
			for i := 110; i < 115; i++ {
				dedicatedNFS[TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i))] = struct{}{}
			}
			// And the global
			globalNFS := map[TResourceName]struct{}{}
			for i := 115; i < 117; i++ {
				globalNFS[TResourceName(fmt.Sprintf("global-nfs-instance-%v", i))] = struct{}{}
			}
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud:                     vms,
				TResourceType("default-nfs"):   defaultNFS,
				TResourceType("dedicated-nfs"): dedicatedNFS,
				TResourceType("global-nfs"):    globalNFS,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// No files removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 15+5+2)
	for i := 100; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
	for i := 110; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
	}
	for i := 115; i < 117; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
	}
}

func TestHandleFilesNoPruneWithNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 10 files, 5 existing and 5 new
	for i := 100; i < 110; i++ {
		newFile := false
		if i >= 105 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 5)
	require.Len(t, tfFiles, 5)

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return the 5 existing instances
			result := map[TResourceName]struct{}{}
			for i := 100; i < 105; i++ {
				result[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
			}
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: result,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// New files renamed, nothing removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 10)
	for i := 100; i < 110; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilesPruneMultipleVMTypes(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 9 files, 3 different VM types
	for i := 100; i < 109; i++ {
		var resType TResourceType
		if i < 103 {
			resType = VMIBMCloud
		} else if i < 106 {
			resType = VMAmazon
		} else {
			resType = VMGoogleCloud
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: resType,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: false,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 9)

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return all of 1 type, 1 of another type, 0 of the last type
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: {
					TResourceName("instance-100"): {},
					TResourceName("instance-101"): {},
					TResourceName("instance-102"): {},
				},
				VMAmazon: {
					TResourceName("instance-103"): {},
				},
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// New files renamed, instances 104+ removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 4)
	for i := 100; i < 104; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilesPruneWithNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 10 files, 5 existing and 5 new
	for i := 100; i < 110; i++ {
		newFile := false
		if i >= 105 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 5)
	require.Len(t, tfFiles, 5)

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return only 3 of the five existing
			result := map[TResourceName]struct{}{
				TResourceName("instance-102"): {},
				TResourceName("instance-103"): {},
				TResourceName("instance-104"): {},
			}
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: result,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// New files renamed, instances 100, 101 remvoed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 8)
	for i := 102; i < 110; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilesDedicatedGlobalNoPruneWithNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 5 files with VMs, 5 with VMs and a default, 5 with VMs
	// and dedicated, and a global. Every even file is new.
	for i := 100; i < 105; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 105; i < 110; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
				{
					ResType: TResourceType("default-nfs"),
					ResName: TResourceName(fmt.Sprintf("default-nfs-instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 110; i < 115; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
		// Dedicated
		info = fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("dedicated-nfs"),
					ResName: TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)),
				},
			},
			FilePrefix: fmt.Sprintf("dedicated-nfs-instance-%v", i),
			NewFile:    newFile,
			Plugin:     tf,
		}
		writeFile(info, t)
	}
	for i := 115; i < 117; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		// Global
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("global-nfs"),
					ResName: TResourceName(fmt.Sprintf("global-nfs-instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	// 100, 102, 104, 106, 108, 110, 112, 114 VMs
	// 110, 112, 114 dedicated
	// 116 global
	require.Len(t, tfFilesNew, 8+3+1)
	for i := 100; i < 115; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("instance-%v.tf.json.new", i))
		}
	}
	for i := 110; i < 115; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json.new", i))
		}
	}
	for i := 115; i < 117; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("global-nfs-instance-%v.tf.json.new", i))
		}
	}
	require.Len(t, tfFiles, 22-(8+3+1))
	for i := 100; i < 115; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
		}
	}
	for i := 110; i < 115; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
		}
	}
	for i := 115; i < 117; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
		}
	}

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return only existing instances (ie, only odd ones)
			vms := map[TResourceName]struct{}{}
			for i := 100; i < 115; i++ {
				if i%2 != 0 {
					vms[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
				}
			}
			defaultNFS := map[TResourceName]struct{}{}
			for i := 105; i < 110; i++ {
				if i%2 != 0 {
					defaultNFS[TResourceName(fmt.Sprintf("default-nfs-instance-%v", i))] = struct{}{}
				}
			}
			dedicatedNFS := map[TResourceName]struct{}{}
			for i := 110; i < 115; i++ {
				if i%2 != 0 {
					dedicatedNFS[TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i))] = struct{}{}
				}
			}
			globalNFS := map[TResourceName]struct{}{}
			for i := 115; i < 117; i++ {
				if i%2 != 0 {
					globalNFS[TResourceName(fmt.Sprintf("global-nfs-instance-%v", i))] = struct{}{}
				}
			}
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud:                     vms,
				TResourceType("default-nfs"):   defaultNFS,
				TResourceType("dedicated-nfs"): dedicatedNFS,
				TResourceType("global-nfs"):    globalNFS,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// No files removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 15+5+2)
	for i := 100; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
	for i := 110; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
	}
	for i := 115; i < 117; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
	}
}

func TestHandleFilesDedicatedGlobalPruneWithNewFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out 5 files with VMs, 5 with VMs and a default, 5 with VMs
	// and dedicated, and a global. Every even file is new.
	for i := 100; i < 105; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 105; i < 110; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
				{
					ResType: TResourceType("default-nfs"),
					ResName: TResourceName(fmt.Sprintf("default-nfs-instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	for i := 110; i < 115; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: VMIBMCloud,
					ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
		// Dedicated
		info = fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("dedicated-nfs"),
					ResName: TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)),
				},
			},
			FilePrefix: fmt.Sprintf("dedicated-nfs-instance-%v", i),
			NewFile:    newFile,
			Plugin:     tf,
		}
		writeFile(info, t)
	}
	for i := 115; i < 117; i++ {
		newFile := false
		if i%2 == 0 {
			newFile = true
		}
		// Global
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType: TResourceType("global-nfs"),
					ResName: TResourceName(fmt.Sprintf("global-nfs-instance-%v", i)),
				},
			},
			NewFile: newFile,
			Plugin:  tf,
		}
		writeFile(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	// 100, 102, 104, 106, 108, 110, 112, 114 VMs
	// 110, 112, 114 dedicated
	// 116 global
	require.Len(t, tfFilesNew, 8+3+1)
	for i := 100; i < 115; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("instance-%v.tf.json.new", i))
		}
	}
	for i := 110; i < 115; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json.new", i))
		}
	}
	for i := 115; i < 117; i++ {
		if i%2 == 0 {
			require.Contains(t, tfFilesNew, fmt.Sprintf("global-nfs-instance-%v.tf.json.new", i))
		}
	}
	require.Len(t, tfFiles, 22-(8+3+1))
	for i := 100; i < 115; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
		}
	}
	for i := 110; i < 115; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
		}
	}
	for i := 115; i < 117; i++ {
		if i%2 != 0 {
			require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
		}
	}

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			// Return only existing instances (ie, only odd ones) EXCEPT 1 in each group
			vms := map[TResourceName]struct{}{}
			for i := 100; i < 115; i++ {
				// 105 will not return for both the VM and NFS, 107 will not return for the VM only
				if i%2 != 0 && i != 105 && i != 107 {
					vms[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
				}
			}
			defaultNFS := map[TResourceName]struct{}{}
			for i := 105; i < 110; i++ {
				if i%2 != 0 && i != 105 {
					defaultNFS[TResourceName(fmt.Sprintf("default-nfs-instance-%v", i))] = struct{}{}
				}
			}
			// Do not return anything for dedicated/global since they are not valid for pruning
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud:                   vms,
				TResourceType("default-nfs"): defaultNFS,
			}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	// No files removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 15-2+5+2)
	for i := 100; i < 115; i++ {
		// 105 has both the VM and default NFS removed, 107 has the VM removed so the entire file is removed
		if i == 105 || i == 107 {
			continue
		}
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
	for i := 110; i < 115; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
	}
	for i := 115; i < 117; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("global-nfs-instance-%v.tf.json", i))
	}
}

func TestHandleFilesDuplicates(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Create 10 dedicated files
	for i := 100; i < 110; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType:  TResourceType("dedicated-nfs"),
					ResName:  TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)),
					ResProps: TResourceProperties{"key": "val"},
				},
			},
			FilePrefix: fmt.Sprintf("dedicated-nfs-instance-%v", i),
			NewFile:    true,
			Plugin:     tf,
		}
		writeFile(info, t)
	}

	fns := tfFuncs{
		tfRefresh: func() error { return nil },
		tfStateList: func(dir string) (map[TResourceType]map[TResourceName]struct{}, error) {
			return map[TResourceType]map[TResourceName]struct{}{}, nil
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)

	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 10)
	for i := 100; i < 110; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
		buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i)))
		require.NoError(t, err)
		tFormat := TFormat{}
		err = types.AnyBytes(buff).Decode(&tFormat)
		require.NoError(t, err)
		require.Equal(t,
			map[TResourceType]map[TResourceName]TResourceProperties{
				TResourceType("dedicated-nfs"): {
					TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)): {
						"key": "val",
					},
				},
			},
			tFormat.Resource,
		)
	}

	// Update 5 of them
	for i := 105; i < 110; i++ {
		info := fileInfo{
			ResInfo: []resInfo{
				{
					ResType:  TResourceType("dedicated-nfs"),
					ResName:  TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)),
					ResProps: TResourceProperties{"key-update": "val-update"},
				},
			},
			FilePrefix: fmt.Sprintf("dedicated-nfs-instance-%v", i),
			NewFile:    true,
			Plugin:     tf,
		}
		writeFile(info, t)
	}
	err = tf.handleFiles(fns)
	require.NoError(t, err)

	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 10)
	for i := 100; i < 110; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i))
		buff, err := ioutil.ReadFile(filepath.Join(tf.Dir, fmt.Sprintf("dedicated-nfs-instance-%v.tf.json", i)))
		require.NoError(t, err)
		tFormat := TFormat{}
		err = types.AnyBytes(buff).Decode(&tFormat)
		require.NoError(t, err)
		props := TResourceProperties{"key": "val"}
		if i >= 105 {
			props = TResourceProperties{"key-update": "val-update"}
		}
		require.Equal(t,
			map[TResourceType]map[TResourceName]TResourceProperties{
				TResourceType("dedicated-nfs"): {
					TResourceName(fmt.Sprintf("dedicated-nfs-instance-%v", i)): props,
				},
			},
			tFormat.Resource,
		)
	}
}
