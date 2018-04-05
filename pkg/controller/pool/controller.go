package pool

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	pool "github.com/docker/infrakit/pkg/controller/pool/types"
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
	log     = logutil.New("module", "controller/pool")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)

	// DefaultModelProperties is the default properties for the fsm model
	DefaultModelProperties = pool.ModelProperties{
		TickUnit:            types.FromDuration(1 * time.Second),
		WaitBeforeProvision: fsm.Tick(3),
		WaitBeforeDestroy:   fsm.Tick(3),
		ChannelBufferSize:   4096,
		Options: fsm.Options{
			Name:                       "pool",
			BufferSize:                 4096,
			IgnoreUndefinedTransitions: true,
			IgnoreUndefinedSignals:     true,
			IgnoreUndefinedStates:      true,
		},
	}

	// DefaultOptions is the default options of the controller. This can be controlled at starup
	// and is set once.
	DefaultOptions = pool.Options{
		InstanceObserver: &internal.InstanceObserver{
			ObserveInterval: types.Duration(5 * time.Second),
			KeySelector: template.EscapeString(fmt.Sprintf(`{{.Tags.%s}}`,
				internal.InstanceLabel)),
		},
		PluginRetryInterval:    types.Duration(1 * time.Second),
		MinChannelBufferSize:   1024,
		MinWaitBeforeProvision: 3,
		ModelProperties:        DefaultModelProperties,
		ProvisionDeadline:      types.Duration(1 * time.Second),
		DestroyDeadline:        types.Duration(1 * time.Second),
	}

	// DefaultProperties is the default properties for the controller, this is per collection / commit
	DefaultProperties = pool.Properties{
		Parallelism: 1,
	}
)

// Components contains a set of components in this controller.
type Components struct {
	Controllers func() (map[string]controller.Controller, error)
	Metadata    func() (map[string]metadata.Plugin, error)
	Events      event.Plugin
}

// NewComponents returns a controller implementation
func NewComponents(scope scope.Scope, options pool.Options) *Components {

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
