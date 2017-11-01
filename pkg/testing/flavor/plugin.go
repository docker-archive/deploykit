package flavor

import (
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements flavor.Plugin and allows dynamically setting the behavior for tests
type Plugin struct {

	// DoValidate implements Validate via function
	DoValidate func(flavorProperties *types.Any, allocation group.AllocationMethod) error

	// DoPrepare implements Prepare via function
	DoPrepare func(flavorProperties *types.Any, spec instance.Spec,
		allocation group.AllocationMethod, index group.Index) (instance.Spec, error)

	// DoHealthy implements Healthy via function
	DoHealthy func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error)

	// DoDrain implements Drain via function
	DoDrain func(flavorProperties *types.Any, inst instance.Description) error
}

// Validate checks whether the helper can support a configuration.
func (t *Plugin) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {
	return t.DoValidate(flavorProperties, allocation)
}

// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
// the flavor configuration.
func (t *Plugin) Prepare(flavorProperties *types.Any,
	spec instance.Spec,
	allocation group.AllocationMethod,
	index group.Index) (instance.Spec, error) {

	return t.DoPrepare(flavorProperties, spec, allocation, index)
}

// Healthy determines the Health of this Flavor on an instance.
func (t *Plugin) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
	return t.DoHealthy(flavorProperties, inst)
}

// Drain allows the flavor to perform a best-effort cleanup operation before the instance is destroyed.
func (t *Plugin) Drain(flavorProperties *types.Any, inst instance.Description) error {
	return t.DoDrain(flavorProperties, inst)
}
