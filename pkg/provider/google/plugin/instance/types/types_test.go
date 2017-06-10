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
		"Disks":[{
			"Image":"docker-image",
			"SizeGb":2000,
			"Type":"pd-ssd",
			"AutoDelete":false,
			"ReuseExisting":true
		}],
		"Scopes":["SCOPE1", "SCOPE2"],
		"TargetPools":["POOL1", "POOL2"],
		"Preemptible":true,
		"Description":"vm"}`)

	p, err := ParseProperties(properties)

	require.NoError(t, err)

	require.Equal(t, "worker", p.NamePrefix)
	require.Equal(t, "vm", p.Description)
	require.Equal(t, "n1-standard-1", p.MachineType)
	require.Equal(t, "NETWORK", p.Network)
	require.Equal(t, true, p.Preemptible)
	require.Equal(t, []string{"TAG1", "TAG2"}, p.Tags)
	require.Equal(t, []string{"SCOPE1", "SCOPE2"}, p.Scopes)
	require.Equal(t, []string{"POOL1", "POOL2"}, p.TargetPools)

	// Disk settings
	bootDisk := p.Disks[0]
	require.Equal(t, "docker-image", bootDisk.Image)
	require.Equal(t, 2000, int(bootDisk.SizeGb))
	require.Equal(t, "pd-ssd", bootDisk.Type)
	require.Equal(t, false, bootDisk.AutoDelete)
	require.Equal(t, true, bootDisk.ReuseExisting)
}

func TestParseEmptyProperties(t *testing.T) {
	properties := types.AnyString(`{}`)

	p, err := ParseProperties(properties)

	require.NoError(t, err)

	require.Equal(t, defaultNamePrefix, p.NamePrefix)
	require.Equal(t, defaultDescription, p.Description)
	require.Equal(t, defaultMachineType, p.MachineType)
	require.Equal(t, defaultNetwork, p.Network)
	require.Equal(t, defaultPreemptible, p.Preemptible)
	require.Nil(t, p.Tags)
	require.Nil(t, p.Scopes)
	require.Nil(t, p.TargetPools)

	// Disk
	bootDisk := p.Disks[0]
	require.Equal(t, defaultDiskImage, bootDisk.Image)
	require.Equal(t, defaultDiskType, bootDisk.Type)
	require.Equal(t, defaultDiskSizeGb, bootDisk.SizeGb)
	require.Equal(t, true, bootDisk.AutoDelete)
	require.Equal(t, false, bootDisk.ReuseExisting)
}

func TestParseDefaultDiskProperties(t *testing.T) {
	properties := types.AnyString(`{
		"Disks":[{
			"Image":"docker-image"
		}]}`)

	p, err := ParseProperties(properties)

	require.NoError(t, err)

	bootDisk := p.Disks[0]
	require.Equal(t, "docker-image", bootDisk.Image)
	require.Equal(t, defaultDiskType, bootDisk.Type)
	require.Equal(t, defaultDiskSizeGb, bootDisk.SizeGb)
	require.Equal(t, true, bootDisk.AutoDelete)
	require.Equal(t, false, bootDisk.ReuseExisting)
}
