package flavor

import (
	"github.com/docker/libmachete/plugin"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
)

type client struct {
	c plugin.Callable
}

// PluginClient returns an instance of the Plugin
func PluginClient(c plugin.Callable) flavor.Plugin {
	return &client{c: c}
}

// Validate checks whether the helper can support a configuration.
func (c *client) Validate(config group.Configuration, parsed types.Schema) error {
	return nil
}

// FlavorOf translates the helper's role names into Roles that define how the group is managed.  This allows
// a helper to define specialized roles and customize those machines accordingly in PreProvision().
// TODO(wfarner): Consider removing this once FlavorPlugin is a first-class configuration entity.
func (c *client) FlavorOf(roleName string) flavor.GroupFlavor {
	return flavor.Unknown
}

// PreProvision allows the helper to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized BootScript based on
// the machine configuration.
func (c *client) PreProvision(config group.Configuration, spec instance.Spec) (instance.Spec, error) {
	return spec, nil
}

// Healthy determines whether an instance is healthy.
func (c *client) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}
