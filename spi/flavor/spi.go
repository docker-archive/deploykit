package flavor

import (
	"encoding/json"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/instance"
)

// InstanceIDKind is the kind of instance identifier for a group.
type InstanceIDKind int

// Definitions of supported instance identifier kinds.
const (
	IDKindUnknown InstanceIDKind = iota
	IDKindPhysical
	IDKindPhysicalWithLogical
)

// Plugin defines custom behavior for what runs on instances.
type Plugin interface {

	// Validate checks whether the helper can support a configuration, and returns the allocation kind required.
	Validate(flavorProperties json.RawMessage, parsed types.Schema) (InstanceIDKind, error)

	// PreProvision allows the helper to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized BootScript based on
	// the machine configuration.
	PreProvision(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error)

	// Healthy determines whether an instance is healthy.
	Healthy(inst instance.Description) (bool, error)
}
