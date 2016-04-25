package command

import (
	"github.com/docker/libmachete/provisioners"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDestroy(t *testing.T) {
	cmd := destroyCmd(provisioners.NewRegistry(map[string]provisioners.ProvisionerBuilder{}))
	require.Nil(t, cmd.RunE(cmd, []string{}))
}
