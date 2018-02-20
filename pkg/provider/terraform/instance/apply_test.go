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

	terraform_types "github.com/docker/infrakit/pkg/provider/terraform/instance/types"
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
	options := terraform_types.Options{
		Dir:          dir,
		PollInterval: types.FromDuration(2 * time.Minute),
	}
	terraform, err := NewTerraformInstancePlugin(options, nil)
	require.NoError(t, err)
	require.False(t, terraform.(*plugin).pretend)
	p, _ := terraform.(*plugin)
	err = p.terraform.doTerraformApply()
	require.NoError(t, err)
}

func TestContinuePollingStandalone(t *testing.T) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	options := terraform_types.Options{
		Dir:          dir,
		Standalone:   true,
		PollInterval: types.FromDuration(2 * time.Minute),
	}

	terraform, err := newPlugin(options, nil, false, getTfLookup(&FakeTerraform{}, nil))
	require.NoError(t, err)
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
func writeFileInfo(info fileInfo, t *testing.T) {
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
	err = writeFileRaw(info.Plugin, filename, buff)
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

func TestHandleFilesStateListBeforeFail(t *testing.T) {
	refreshInvoked := false
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			refreshInvoked = true
			return nil
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			return nil, fmt.Errorf("Custom state list error")
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
	defer os.RemoveAll(dir)

	err := tf.handleFiles(tfFuncs{})
	require.Error(t, err)
	require.Equal(t, "Custom state list error", err.Error())
	require.False(t, refreshInvoked)
}

func TestHandleFilesRefreshFail(t *testing.T) {
	stateListInvokedCount := 0
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			return fmt.Errorf("Custom refresh error")
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			stateListInvokedCount = stateListInvokedCount + 1
			return map[TResourceType]map[TResourceName]struct{}{}, nil
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
	}

	err := tf.handleFiles(tfFuncs{})
	require.Error(t, err)
	require.Equal(t, "Custom refresh error", err.Error())
	require.Equal(t, 1, stateListInvokedCount)

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

func TestHandleFilesStateListAfterFail(t *testing.T) {
	stateListInvokedCount := 0
	refreshInvoked := false
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			refreshInvoked = true
			return nil
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			stateListInvokedCount = stateListInvokedCount + 1
			if stateListInvokedCount == 1 {
				return map[TResourceType]map[TResourceName]struct{}{}, nil
			}
			return nil, fmt.Errorf("Custom state list error")
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
	defer os.RemoveAll(dir)

	err := tf.handleFiles(tfFuncs{})
	require.Error(t, err)
	require.Equal(t, "Custom state list error", err.Error())
	require.True(t, refreshInvoked)
	require.Equal(t, 2, stateListInvokedCount)
}

func TestHandleFilesNoFiles(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	err := tf.handleFiles(tfFuncs{})
	require.NoError(t, err)
}

func TestHasRecentDelta(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// No files (so no deltas)
	hasDelta, err := tf.hasRecentDeltas(60)
	require.NoError(t, err)
	require.False(t, hasDelta)

	// Non tf.json[.new] file (should be ignored)
	err = writeFileRaw(tf, "foo.txt", []byte("some-text"))
	require.NoError(t, err)
	hasDelta, err = tf.hasRecentDeltas(60)
	require.NoError(t, err)
	require.False(t, hasDelta)

	// Write out a tf.json file
	info := fileInfo{
		ResInfo: []resInfo{
			{
				ResType: VMIBMCloud,
				ResName: TResourceName("instance-12345"),
			},
		},
		NewFile: false,
		Plugin:  tf,
	}
	writeFileInfo(info, t)

	// File delta in the last 5 seconds
	hasDelta, err = tf.hasRecentDeltas(5)
	require.NoError(t, err)
	require.True(t, hasDelta)

	// Wait 2 seconds, now the file will not a delta in a 1 second window
	time.Sleep(2 * time.Second)
	hasDelta, err = tf.hasRecentDeltas(1)
	require.NoError(t, err)
	require.False(t, hasDelta)

	// But not if we ask for deltas in a longer window
	hasDelta, err = tf.hasRecentDeltas(60)
	require.NoError(t, err)
	require.True(t, hasDelta)

	// Add another, should be a delta
	info.ResInfo[0].ResName = TResourceName("instance-12346")
	writeFileInfo(info, t)
	hasDelta, err = tf.hasRecentDeltas(1)
	require.NoError(t, err)
	require.True(t, hasDelta)
}

func TestHasRecentDeltaInFuture(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Write out a tf.json file
	info := fileInfo{
		ResInfo: []resInfo{
			{
				ResType: VMIBMCloud,
				ResName: TResourceName("instance-12345"),
			},
		},
		NewFile: false,
		Plugin:  tf,
	}
	writeFileInfo(info, t)

	// Update the timestamp to 29 seconds in the future
	path := filepath.Join(tf.Dir, "instance-12345.tf.json")
	newTime := time.Now().Add(time.Duration(29) * time.Second)
	err := tf.fs.Chtimes(path, newTime, newTime)
	require.NoError(t, err)

	// Since it's less than 30 seconds it will be a delta
	hasDelta, err := tf.hasRecentDeltas(1)
	require.NoError(t, err)
	require.True(t, hasDelta)

	// More than 30 seconds will be ignored
	newTime = time.Now().Add(time.Duration(35) * time.Second)
	err = tf.fs.Chtimes(path, newTime, newTime)
	require.NoError(t, err)
	hasDelta, err = tf.hasRecentDeltas(1)
	require.NoError(t, err)
	require.False(t, hasDelta)
}

func TestHandleFilesNoPruneNoNewFiles(t *testing.T) {
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			return nil
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
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
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 5)

	err := tf.handleFiles(tfFuncs{})
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
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			return nil
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
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
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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

	err := tf.handleFiles(tfFuncs{})
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
	fake := FakeTerraform{
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
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
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 5)
	require.Len(t, tfFiles, 5)

	err := tf.handleFiles(tfFuncs{})
	require.NoError(t, err)

	// New files renamed, nothing removed
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 10)
	for i := 100; i < 110; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilePruningNoPrunes(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	err := tf.handleFilePruning(tfFuncs{},
		map[TResourceType]map[TResourceName]TResourceFilenameProps{},
		map[TResourceType]map[TResourceName]struct{}{})
	require.NoError(t, err)
}

func TestHandleFilePruningPruneExistingResourceError(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	fns := tfFuncs{
		getExistingResource: func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
			require.Equal(t, VMIBMCloud, resType)
			require.Equal(t, TResourceName("instance-123"), resName)
			require.Equal(t, TResourceProperties{"foo": "bar"}, props)
			return nil, fmt.Errorf("Custom getExistingResource error")
		},
	}
	err := tf.handleFilePruning(fns,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			VMIBMCloud: {
				TResourceName("instance-123"): {
					FileName:  "instance-123.tf.json",
					FileProps: TResourceProperties{"foo": "bar"},
				},
			},
		},
		map[TResourceType]map[TResourceName]struct{}{})
	require.Error(t, err)
	require.Equal(t, "Custom getExistingResource error", err.Error())
}

func TestHandleFilePruningPruneImportError(t *testing.T) {
	fake := FakeTerraform{
		doTerraformImportStub: func(fs afero.Fs, resType TResourceType, resName, resID string, createDummyFile bool) error {
			require.Equal(t, VMIBMCloud, resType)
			require.Equal(t, "instance-123", resName)
			require.Equal(t, "some-id", resID)
			require.False(t, createDummyFile)
			return fmt.Errorf("Custom import error")
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
	defer os.RemoveAll(dir)

	fns := tfFuncs{
		getExistingResource: func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
			id := "some-id"
			return &id, nil
		},
	}
	err := tf.handleFilePruning(fns,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			VMIBMCloud: {
				TResourceName("instance-123"): {
					FileName:  "instance-123.tf.json",
					FileProps: TResourceProperties{"foo": "bar"},
				},
			},
		},
		map[TResourceType]map[TResourceName]struct{}{})
	require.Error(t, err)
	require.Equal(t, "Custom import error", err.Error())
}

func TestHandleFilePruningRemovedFromBackend(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	// Create file, should be removed since it is not in the backend
	info := fileInfo{
		ResInfo: []resInfo{{ResType: VMIBMCloud, ResName: TResourceName("instance-123")}},
		NewFile: false,
		Plugin:  tf,
	}
	writeFileInfo(info, t)
	// Random file, should not be removed
	info = fileInfo{
		ResInfo: []resInfo{{ResType: VMIBMCloud, ResName: TResourceName("instance-234")}},
		NewFile: false,
		Plugin:  tf,
	}
	writeFileInfo(info, t)

	fns := tfFuncs{
		getExistingResource: func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
			// Resource is not in the backend
			return nil, nil
		},
	}
	err := tf.handleFilePruning(fns,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			VMIBMCloud: {
				TResourceName("instance-123"): {
					FileName:  "instance-123.tf.json",
					FileProps: TResourceProperties{"foo": "bar"},
				},
			},
		},
		map[TResourceType]map[TResourceName]struct{}{})
	require.NoError(t, err)

	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 1)
	require.Contains(t, tfFiles, "instance-234.tf.json")
}

func TestHandleFilePruningImportSuccess(t *testing.T) {
	fake := FakeTerraform{
		doTerraformImportStub: func(fs afero.Fs, resType TResourceType, resName, resID string, createDummyFile bool) error {
			// Import is successful
			require.False(t, createDummyFile)
			return nil
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
	defer os.RemoveAll(dir)

	fns := tfFuncs{
		getExistingResource: func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
			id := "some-id"
			return &id, nil
		},
	}
	err := tf.handleFilePruning(fns,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			VMIBMCloud: {
				TResourceName("instance-123"): {
					FileName:  "instance-123.tf.json",
					FileProps: TResourceProperties{"foo": "bar"},
				},
			},
		},
		map[TResourceType]map[TResourceName]struct{}{})
	require.NoError(t, err)
}

func TestGetExistingResourceNoVMs(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	id, err := tf.getExistingResource(TResourceType("storage"), TResourceName("name"), TResourceProperties{})
	require.Nil(t, id)
	require.NoError(t, err)
}

func TestGetExistingResourceUnsupportedType(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	id, err := tf.getExistingResource(VMAzure, TResourceName("name"), TResourceProperties{})
	require.Nil(t, id)
	require.NoError(t, err)
}

func TestGetExistingResourceIBMCloudNoTags(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	id, err := tf.getExistingResource(VMIBMCloud, TResourceName("name"), TResourceProperties{})
	require.Nil(t, id)
	require.NoError(t, err)
}

func TestGetExistingResourceIBMCloudWrongTagType(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)

	id, err := tf.getExistingResource(VMIBMCloud, TResourceName("name"), TResourceProperties{"tags": "string"})
	require.Nil(t, id)
	require.Error(t, err)
	require.Equal(t, "Cannot process tags, unknown type: string", err.Error())
}

func TestGetExistingResourceIBMCloudWrongCreds(t *testing.T) {
	tf, dir := getPlugin(t)
	defer os.RemoveAll(dir)
	// User bogus creds, will always get an error
	tf.envs = []string{
		SoftlayerUsernameEnvVar + "=user",
		SoftlayerAPIKeyEnvVar + "=pass",
	}
	os.Setenv(SoftlayerUsernameEnvVar, "")
	os.Setenv(SoftlayerAPIKeyEnvVar, "")

	id, err := tf.getExistingResource(VMIBMCloud, TResourceName("name"), TResourceProperties{"tags": []interface{}{"t1", "t2"}})
	require.Nil(t, id)
	require.Error(t, err)
}

const (
	Prune1RemoveOutOfBand   = 1
	Prune2ExistsInBackend   = 2
	Prune3NoExistsInBackend = 3
)

func TestHandleFilesPruneMultipleVMTypes_OutOfBand(t *testing.T) {
	internalTestHandleFilesPruneMultipleVMTypes(t, Prune1RemoveOutOfBand)
}

func TestHandleFilesPruneMultipleVMTypes_ExistsInBackend(t *testing.T) {
	internalTestHandleFilesPruneMultipleVMTypes(t, Prune2ExistsInBackend)
}

func TestHandleFilesPruneMultipleVMTypes_NoExistsInBackend(t *testing.T) {
	internalTestHandleFilesPruneMultipleVMTypes(t, Prune3NoExistsInBackend)
}

func internalTestHandleFilesPruneMultipleVMTypes(t *testing.T, pruneType int) {
	stateListInvokedCount := 0
	fake := FakeTerraform{
		doTerraformRefreshStub: func() error {
			return nil
		},
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			stateListInvokedCount = stateListInvokedCount + 1
			// Base resources that are not removed
			result := map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: {
					TResourceName("instance-100"): {},
					TResourceName("instance-101"): {},
					TResourceName("instance-102"): {},
				},
				VMAmazon: {
					TResourceName("instance-103"): {},
				},
			}
			if stateListInvokedCount == 1 {
				// Files are removed out-of-band, return them the first time but
				// not the second time
				if pruneType == Prune1RemoveOutOfBand {
					result[VMAmazon][TResourceName("instance-104")] = struct{}{}
					result[VMAmazon][TResourceName("instance-105")] = struct{}{}
					result[VMGoogleCloud] = map[TResourceName]struct{}{
						TResourceName("instance-106"): {},
						TResourceName("instance-107"): {},
						TResourceName("instance-108"): {},
					}
				}
			}
			// Return all of 1 type, 1 of another type, 0 of the last type
			return result, nil
		},
		doTerraformImportStub: func(fs afero.Fs, resType TResourceType, resName, resID string, createDummyFile bool) error {
			// Should only invoked if the resources exist in the backend
			if pruneType == Prune2ExistsInBackend {
				if resType == VMAmazon && resName == "instance-105" {
					require.Equal(t, "aws-instance-105", resID)
					return nil
				}
				if resType == VMGoogleCloud && resName == "instance-108" {
					require.Equal(t, "gcp-instance-108", resID)
					return nil
				}
				return fmt.Errorf("tfImport type %v, name %v, id %v", resType, resName, resID)
			}
			return fmt.Errorf("tfImport should not be invoked for pruneType: %v", pruneType)
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	require.Len(t, tfFiles, 9)

	fns := tfFuncs{
		getExistingResource: func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
			if pruneType == Prune1RemoveOutOfBand {
				return nil, fmt.Errorf("tfImport should not be invoked for pruneType: Prune1RemoveOutOfBand")
			}
			if pruneType == Prune2ExistsInBackend {
				if resType == VMAmazon && resName == TResourceName("instance-105") {
					id := "aws-instance-105"
					return &id, nil
				}
				if resType == VMGoogleCloud && resName == TResourceName("instance-108") {
					id := "gcp-instance-108"
					return &id, nil
				}
				return nil, nil
			}
			if pruneType == Prune3NoExistsInBackend {
				return nil, nil
			}
			return nil, fmt.Errorf("UNKNOWN PRUNE TYPE")
		},
	}
	err := tf.handleFiles(fns)
	require.NoError(t, err)
	require.Equal(t, 2, stateListInvokedCount)

	// New files renamed, instances 104+ removed if the resources are not in the backend
	tfFiles, tfFilesNew = getFilenames(t, tf)
	require.Len(t, tfFilesNew, 0)
	if pruneType == Prune2ExistsInBackend {
		require.Len(t, tfFiles, 6)
		require.Contains(t, tfFiles, "instance-105.tf.json")
		require.Contains(t, tfFiles, "instance-108.tf.json")
	} else {
		require.Len(t, tfFiles, 4)
	}
	for i := 100; i < 104; i++ {
		require.Contains(t, tfFiles, fmt.Sprintf("instance-%v.tf.json", i))
	}
}

func TestHandleFilesPruneWithNewFiles(t *testing.T) {
	stateListInvokedCount := 0
	fake := FakeTerraform{
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			stateListInvokedCount = stateListInvokedCount + 1
			// Return everything the first time
			if stateListInvokedCount == 1 {
				return map[TResourceType]map[TResourceName]struct{}{
					VMIBMCloud: {
						TResourceName("instance-100"): {},
						TResourceName("instance-101"): {},
						TResourceName("instance-102"): {},
						TResourceName("instance-103"): {},
						TResourceName("instance-104"): {},
					},
				}, nil
			}
			// Return only 3 of the five existing
			return map[TResourceType]map[TResourceName]struct{}{
				VMIBMCloud: {
					TResourceName("instance-102"): {},
					TResourceName("instance-103"): {},
					TResourceName("instance-104"): {},
				},
			}, nil
		},
	}
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
	}
	tfFiles, tfFilesNew := getFilenames(t, tf)
	require.Len(t, tfFilesNew, 5)
	require.Len(t, tfFiles, 5)

	err := tf.handleFiles(tfFuncs{})
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
	fake := FakeTerraform{
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
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
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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

	err := tf.handleFiles(tfFuncs{})
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
	stateListInvokedCount := 0
	fake := FakeTerraform{
		doTerraformStateListStub: func() (map[TResourceType]map[TResourceName]struct{}, error) {
			stateListInvokedCount = stateListInvokedCount + 1
			// Return only existing instances (ie, only odd ones) EXCEPT 1 in each group
			vms := map[TResourceName]struct{}{}
			for i := 100; i < 115; i++ {
				// 105 will not return for both the VM and NFS, 107 will not return for the VM only
				if i%2 != 0 && (stateListInvokedCount == 1 || (i != 105 && i != 107)) {
					vms[TResourceName(fmt.Sprintf("instance-%v", i))] = struct{}{}
				}
			}
			defaultNFS := map[TResourceName]struct{}{}
			for i := 105; i < 110; i++ {
				if i%2 != 0 && (stateListInvokedCount == 1 || i != 105) {
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
	tf, dir := getPluginWithFakeTerraform(t, &fake)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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
		writeFileInfo(info, t)
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

	err := tf.handleFiles(tfFuncs{})
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
		writeFileInfo(info, t)
	}

	err := tf.handleFiles(tfFuncs{})
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
		writeFileInfo(info, t)
	}
	err = tf.handleFiles(tfFuncs{})
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

func TestAddToResTypeNamePropsMap(t *testing.T) {
	m := make(map[TResourceType]map[TResourceName]TResourceFilenameProps)
	addToResTypeNamePropsMap(m, TResourceType("t1"), TResourceName("n1"), "f1", TResourceProperties{"k1": "v1"})
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			TResourceType("t1"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1",
					FileProps: TResourceProperties{"k1": "v1"},
				},
			},
		},
		m)

	addToResTypeNamePropsMap(m, TResourceType("t1"), TResourceName("n2"), "f2", TResourceProperties{"k2": "v2"})
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			TResourceType("t1"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1",
					FileProps: TResourceProperties{"k1": "v1"},
				},
				TResourceName("n2"): TResourceFilenameProps{
					FileName:  "f2",
					FileProps: TResourceProperties{"k2": "v2"},
				},
			},
		},
		m)

	addToResTypeNamePropsMap(m, TResourceType("t2"), TResourceName("n1"), "f1", TResourceProperties{"k1": "v1"})
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			TResourceType("t1"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1",
					FileProps: TResourceProperties{"k1": "v1"},
				},
				TResourceName("n2"): TResourceFilenameProps{
					FileName:  "f2",
					FileProps: TResourceProperties{"k2": "v2"},
				},
			},
			TResourceType("t2"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1",
					FileProps: TResourceProperties{"k1": "v1"},
				},
			},
		},
		m)

	addToResTypeNamePropsMap(m, TResourceType("t1"), TResourceName("n1"), "f1-new", TResourceProperties{"k3": "v3"})
	require.Equal(t,
		map[TResourceType]map[TResourceName]TResourceFilenameProps{
			TResourceType("t1"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1-new",
					FileProps: TResourceProperties{"k3": "v3"},
				},
				TResourceName("n2"): TResourceFilenameProps{
					FileName:  "f2",
					FileProps: TResourceProperties{"k2": "v2"},
				},
			},
			TResourceType("t2"): {
				TResourceName("n1"): TResourceFilenameProps{
					FileName:  "f1",
					FileProps: TResourceProperties{"k1": "v1"},
				},
			},
		},
		m)
}
