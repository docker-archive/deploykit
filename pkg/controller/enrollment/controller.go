package enrollment

import (
	"github.com/docker/infrakit/pkg/controller"
	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "controller/enrollment")

// NewController returns a controller implementation
func NewController(plugins func() discovery.Plugins, leader manager.Leadership,
	options enrollment.Options) controller.Controller {
	return internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newEnroller(plugins, leader, options), nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
}

// NewTypedControllers return typed controllers
func NewTypedControllers(plugins func() discovery.Plugins, leader manager.Leadership,
	options enrollment.Options) func() (map[string]controller.Controller, error) {

	return (internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newEnroller(plugins, leader, options), nil
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)).ManagedObjects
}

func (l *enroller) started() bool {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.running
}
