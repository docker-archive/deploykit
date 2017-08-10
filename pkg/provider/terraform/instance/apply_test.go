package main

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

// fileInfo holds the data for a file to create in the plugin's working dir
type fileInfo struct {
	ResType TResourceType
	ResName TResourceName
	NewFile bool
	Plugin  *plugin
}

// writeFile is a utility function to write out a terraform file
func writeFile(info fileInfo, t *testing.T) {
	inst := make(map[TResourceType]map[TResourceName]TResourceProperties)
	inst[info.ResType] = map[TResourceName]TResourceProperties{
		info.ResName: {"key": "val"},
	}
	buff, err := json.MarshalIndent(TFormat{Resource: inst}, " ", " ")
	require.NoError(t, err)
	var filename string
	if info.NewFile {
		filename = fmt.Sprintf("%v.tf.json.new", string(info.ResName))
	} else {
		filename = fmt.Sprintf("%v.tf.json", string(info.ResName))
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
			ResType: VMIBMCloud,
			ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
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
			ResType: VMIBMCloud,
			ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
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
			ResType: VMIBMCloud,
			ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
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

func TestHandleFilesPruneMultipleResourceTypes(t *testing.T) {
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
			ResType: resType,
			ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
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
			ResType: VMIBMCloud,
			ResName: TResourceName(fmt.Sprintf("instance-%v", i)),
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
