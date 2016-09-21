package flavor

import (
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
)

// GroupFlavor is the identifier for the type of supervision needed by a group.
type GroupFlavor int

// Definitions of supported role types.
const (
	Unknown   GroupFlavor = iota
	DynamicIP GroupFlavor = iota
	StaticIP  GroupFlavor = iota
)

// Plugin defines custom behavior for what runs on instances.
type Plugin interface {

	// Validate checks whether the helper can support a configuration.
	Validate(config group.Configuration, parsed types.Schema) error

	// FlavorOf translates the helper's role names into Roles that define how the group is managed.  This allows
	// a helper to define specialized roles and customize those machines accordingly in PreProvision().
	// TODO(wfarner): Consider removing this once FlavorPlugin is a first-class configuration entity.
	FlavorOf(roleName string) GroupFlavor

	// PreProvision allows the helper to modify the provisioning instructions for an instance.  For example, a
	// helper could be used to place additional tags on the machine, or generate a specialized BootScript based on
	// the machine configuration.
	PreProvision(config group.Configuration, spec instance.Spec) (instance.Spec, error)

	// Healthy determines whether an instance is healthy.
	Healthy(inst instance.Description) (bool, error)
}
