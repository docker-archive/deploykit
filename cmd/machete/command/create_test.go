package command

import (
	"github.com/docker/libmachete/provisioners"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreate(t *testing.T) {
	cmd := createCmd(provisioners.NewRegistry(map[string]provisioners.ProvisionerBuilder{}))
	require.Nil(t, cmd.RunE(cmd, []string{}))
}
