package resource

import (
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/resource")
	debugV  = logutil.V(200)
	debugV2 = logutil.V(500)

	// DefaultOptions return an Options with default values filled in.
	DefaultOptions = resource.Options{
		PluginRetryInterval: types.Duration(1 * time.Second),
	}
)

// Components contains a set of components in this controller.
type Components struct {
	Controllers func() (map[string]controller.Controller, error)
	Metadata    func() (map[string]metadata.Plugin, error)
}

// NewController returns a controller implementation
func NewController(scope scope.Scope,
	leader func() stack.Leadership, options resource.Options) *Components {

	controller := internal.NewController(
		leader,
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newCollection(scope, leader, options)
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)
	return &Components{
		Controllers: controller.ManagedObjects,
		Metadata:    controller.Metadata,
	}
}
