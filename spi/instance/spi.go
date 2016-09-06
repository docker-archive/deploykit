package instance

import (
	"github.com/docker/libmachete/spi/group"
)

// Plugin is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Plugin interface {
	// Provision creates a new instance.
	Provision(gid group.ID, req string, volume *VolumeID) (*ID, error)

	// Destroy terminates an existing instance.
	Destroy(instance ID) error

	// DescribeInstances returns descriptions of all instances included in a group.
	DescribeInstances(grp group.ID) ([]Description, error)
}
