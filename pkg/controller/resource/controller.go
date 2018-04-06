package resource

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/resource")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)

	// DefaultModelProperties is the default properties for the fsm model
	DefaultModelProperties = resource.ModelProperties{
		TickUnit:            types.FromDuration(1 * time.Second),
		WaitBeforeProvision: fsm.Tick(5),
		WaitBeforeDestroy:   fsm.Tick(5),
		ChannelBufferSize:   4096,
		Options: fsm.Options{
			Name:                       "resource",
			BufferSize:                 4096,
			IgnoreUndefinedTransitions: true,
			IgnoreUndefinedSignals:     true,
			IgnoreUndefinedStates:      true,
		},
	}

	// DefaultOptions is the default options of the controller. This can be controlled at starup
	// and is set once.
	DefaultOptions = resource.Options{
		InstanceObserver: &internal.InstanceObserver{
			CacheDescribeInstances: true,
			ObserveInterval:        types.Duration(5 * time.Second),
			KeySelector: template.EscapeString(fmt.Sprintf(`{{.Tags.%s}}`,
				internal.InstanceLabel)),
		},
		PluginRetryInterval:    types.Duration(1 * time.Second),
		MinChannelBufferSize:   10,
		MinWaitBeforeProvision: 5,
		ModelProperties:        DefaultModelProperties,
		ProvisionDeadline:      types.Duration(1 * time.Second),
		DestroyDeadline:        types.Duration(1 * time.Second),
	}

	// DefaultProperties is the default properties for the controller, this is per collection / commit
	DefaultProperties = resource.Properties{}
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
