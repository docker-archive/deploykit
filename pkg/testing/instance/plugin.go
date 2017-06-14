package instance

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements the instance.Plugin interface and supports testing by letting user assemble behavior dyanmically.
type Plugin struct {
	// DoValidate performs local validation on a provision request.
	DoValidate func(req *types.Any) error

	// DoProvision creates a new instance based on the spec.
	DoProvision func(spec instance.Spec) (*instance.ID, error)

	// DoLabel labels the resource
	DoLabel func(instance instance.ID, labels map[string]string) error

	// DoDestroy terminates an existing instance.
	DoDestroy func(instance instance.ID, context instance.Context) error

	// DoDescribeInstances returns descriptions of all instances matching all of the provided tags.
	DoDescribeInstances func(tags map[string]string, details bool) ([]instance.Description, error)
}

// Validate performs local validation on a provision request.
func (t *Plugin) Validate(req *types.Any) error {
	return t.DoValidate(req)
}

// Provision creates a new instance based on the spec.
func (t *Plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	return t.DoProvision(spec)
}

// Label labels the resource
func (t *Plugin) Label(instance instance.ID, labels map[string]string) error {
	return t.DoLabel(instance, labels)
}

// Destroy terminates an existing instance.
func (t *Plugin) Destroy(instance instance.ID, context instance.Context) error {
	return t.DoDestroy(instance, context)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (t *Plugin) DescribeInstances(tags map[string]string, details bool) ([]instance.Description, error) {
	return t.DoDescribeInstances(tags, details)
}
