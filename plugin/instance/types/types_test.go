package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseProperties(t *testing.T) {
	properties := types.AnyString(`{
		"NamePrefix":"worker",
		"MachineType":"n1-standard-1",
		"Network":"NETWORK",
		"Tags":["TAG1", "TAG2"],
		"DiskImage":"docker-image",
		"Scopes":["SCOPE1", "SCOPE2"],
		"TargetPools":["POOL1", "POOL2"],
		"Preemptible":true,
		"Description":"vm"}`)

	p, err := ParseProperties(properties)
	require.NoError(t, err)
	require.Equal(t, defaultDiskType, p.InstanceSettings.DiskType)
	require.Equal(t, int(defaultDiskSizeMb), int(p.InstanceSettings.DiskSizeMb))
}
