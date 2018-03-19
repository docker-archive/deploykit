package inventory

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	inventory "github.com/docker/infrakit/pkg/controller/inventory/types"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/inventory")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)

	// DefaultOptions return an Options with default values filled in.
	DefaultOptions = inventory.Options{
		InstanceObserver: &internal.InstanceObserver{
			ObserveInterval: types.Duration(5 * time.Second),
			KeySelector: template.EscapeString(fmt.Sprintf(`{{.Tags.%s}}/{{.Tags.%s}}`,
				internal.CollectionLabel, internal.InstanceLabel)),
		},
		PluginRetryInterval:  types.Duration(1 * time.Second),
		MinChannelBufferSize: 10,
		ModelProperties:      DefaultModelProperties,
	}

	// DefaultModelProperties is the default properties for the fsm model
	DefaultModelProperties = inventory.ModelProperties{
		TickUnit:          types.FromDuration(1 * time.Second),
		ChannelBufferSize: 10,
	}

	// DefaultProperties is the default properties for the controller, this is per collection / commit
	DefaultProperties = inventory.Properties{}
)

// Components contains a set of components in this controller.
type Components struct {
	Controllers func() (map[string]controller.Controller, error)
	Metadata    func() (map[string]metadata.Plugin, error)
	Events      event.Plugin
}

// NewComponents returns a controller implementation
func NewComponents(scope scope.Scope, options inventory.Options) *Components {

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
