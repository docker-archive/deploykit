package internal

import (
	"time"

	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/stack"
)

// InstanceAccess is an entity capable of observing an instance plugin
// while providing the same interface methods as an instance Plugin.
// It also contains the fields necessary for provisioning an instance.
type InstanceAccess struct {
	InstanceObserver `json:",inline" yaml:",inline"`

	// Spec is the spec to use when provisioning the instance
	instance.Spec `json:",inline" yaml:",inline"`

	// Plugin exposes the instance.Plugin methods
	instance.Plugin `json:"-"`
}

// Init overrides InstanceObserver.Init() to provide additional initialization.
func (a *InstanceAccess) Init(scope scope.Scope, leader func() stack.Leadership, retry time.Duration) error {
	if err := a.InstanceObserver.Init(scope, leader, retry); err != nil {
		return err
	}
	a.Plugin = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return scope.Instance(a.InstanceObserver.Plugin.String())
		},
		retry)
	return nil
}

// Provision provisions an instance using the spec included in the struct
func (a *InstanceAccess) Provision() (*instance.ID, error) {
	return a.Plugin.Provision(a.Spec)
}
