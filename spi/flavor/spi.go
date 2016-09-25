package flavor

import (
	"encoding/json"
	"github.com/docker/libmachete/spi/instance"
)

// AllocationMethod defines the type of allocation and supervision needed by a flavor's Group.
type AllocationMethod struct {
	Size       uint
	LogicalIDs []instance.LogicalID
}

// Plugin defines custom behavior for what runs on instances.
type Plugin interface {

	// Validate checks whether the helper can support a configuration, and returns the allocation kind required.
	Validate(flavorProperties json.RawMessage) (AllocationMethod, error)

	// PreProvision allows the flavor to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
	// the flavor configuration.
	Prepare(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error)

	// Healthy determines whether an instance is healthy.
	Healthy(inst instance.Description) (bool, error)
}
