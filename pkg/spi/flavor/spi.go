package flavor

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// ClusterIDTag is the name of the tag that contains unique ID for the cluster
	ClusterIDTag = "infrakit.cluster.id"
)

// InterfaceSpec is the current name and version of the Flavor API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Flavor",
	Version: "0.1.1",
}

// Health is an indication of whether the Flavor is functioning properly.
type Health int

const (
	// Unknown indicates that the Health cannot currently be confirmed.
	Unknown Health = iota

	// Healthy indicates that the Flavor is confirmed to be functioning.
	Healthy

	// Unhealthy indicates that the Flavor is confirmed to not be functioning properly.
	Unhealthy
)

// Plugin defines custom behavior for what runs on instances.
type Plugin interface {

	// Validate checks whether the helper can support a configuration.
	Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error

	// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
	// the flavor configuration.
	Prepare(flavorProperties *types.Any, spec instance.Spec,
		allocation group.AllocationMethod,
		createContext group.Index) (instance.Spec, error)

	// Healthy determines the Health of this Flavor on an instance.
	Healthy(flavorProperties *types.Any, inst instance.Description) (Health, error)

	// Drain allows the flavor to perform a best-effort cleanup operation before the instance is destroyed.
	Drain(flavorProperties *types.Any, inst instance.Description) error
}
