package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

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
	terraform := NewTerraformInstancePlugin(dir, 1*time.Second, false, "", "")
	p, _ := terraform.(*plugin)
	attempted, err := p.doTerraformApply(false)
	require.NoError(t, err)
	require.True(t, attempted)
}

func TestContinuePollingStandalone(t *testing.T) {
	dir, err := ioutil.TempDir("", "infrakit-instance-terraform")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	terraform := NewTerraformInstancePlugin(dir, 1*time.Second, true, "", "")
	p, _ := terraform.(*plugin)
	shoudApply := p.shouldApply()
	require.True(t, shoudApply)
}
