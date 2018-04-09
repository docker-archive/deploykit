package types

import (
	"context"

	"github.com/docker/infrakit/pkg/controller/internal"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log    = logutil.New("module", "controller/gc/types")
	debugV = logutil.V(200)
)

func init() {
	depends.Register("gc", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
func ResolveDependencies(spec types.Spec) (depends.Runnables, error) {
	if spec.Properties == nil {
		return nil, nil
	}

	properties := Properties{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, err
	}

	return depends.Runnables{
		depends.AsRunnable(
			types.Spec{
				Kind: properties.InstanceObserver.Name.Lookup(),
				Metadata: types.Metadata{
					Name: properties.InstanceObserver.Name.String(),
				},
			},
		),
		depends.AsRunnable(
			types.Spec{
				Kind: properties.NodeObserver.Name.Lookup(),
				Metadata: types.Metadata{
					Name: properties.NodeObserver.Name.String(),
				},
			},
		),
	}, nil
}

// Properties is the schema of the configuration in the types.Spec.Properties
type Properties struct {

	// Model is the workflow model to use
	Model string

	// ModelProperties contains model-specific configurations
	ModelProperties *types.Any

	// InstanceObserver is the observer of 'instances' side
	InstanceObserver internal.InstanceObserver

	// NodeObserver is the observer of the 'node' side
	NodeObserver internal.InstanceObserver
}

// Validate validates the input properties
func (p Properties) Validate(ctx context.Context) error {
	return nil
}

// Options is the controller options that is used at start up of the process.  It's one-time
type Options struct {

	// PluginRetryInterval is the interval for retrying to connect to the plugins
	PluginRetryInterval types.Duration
}

// Validate validates the controller's options
func (p Options) Validate(ctx context.Context) error {
	return nil
}
