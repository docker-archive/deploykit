package flavor

import (
	"encoding/json"
	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/spi/instance"
)

// Plugin defines custom behavior for what runs on instances.
type Plugin interface {

	// Validate checks whether the helper can support a configuration.
	Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error

	// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
	// the flavor configuration.
	Prepare(flavorProperties json.RawMessage, spec instance.Spec, allocation types.AllocationMethod) (instance.Spec, error)

	// Healthy determines whether an instance is healthy.
	Healthy(inst instance.Description) (bool, error)
}
