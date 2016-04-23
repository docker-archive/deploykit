package cmd

import (
	"github.com/docker/libmachete/provisioners"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreate(t *testing.T) {
	cmd := createCmd(provisioners.NewRegistry(map[string]provisioners.Creator{}))
	require.Nil(t, cmd.RunE(cmd, []string{}))
}
