package resource

import (
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/resource")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)

	// DefaultOptions return an Options with default values filled in.
	DefaultOptions = resource.Options{
		PluginRetryInterval: types.Duration(1 * time.Second),
		LostBufferSize:      10,
		FoundBufferSize:     10,
	}
)

// Components contains a set of components in this controller.
type Components struct {
	Controllers func() (map[string]controller.Controller, error)
	Metadata    func() (map[string]metadata.Plugin, error)
	Events      event.Plugin
}

// NewComponents returns a controller implementation
func NewComponents(scope scope.Scope, options resource.Options) *Components {

	controller := internal.NewController(
		// the constructor
		func(spec types.Spec) (internal.Managed, error) {
			return newCollection(scope, options)
		},
		// the key function
		func(metadata types.Metadata) string {
			return metadata.Name
		},
	)

	return &Components{
		Controllers: controller.Controllers,
		Metadata:    controller.Metadata,
		Events:      controller,
	}
}
