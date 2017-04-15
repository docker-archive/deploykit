package main

import (
	"os"
	"path"
	"testing"

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

	err = doTerraformApply(dir)
	require.NoError(t, err)
}
