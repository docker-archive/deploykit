package main

import (
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
	terraform := NewTerraformInstancePlugin(dir, 1*time.Second)
	p, _ := terraform.(*plugin)
	attempted, err := p.doTerraformApply(false)
	require.NoError(t, err)
	require.True(t, attempted)
}
