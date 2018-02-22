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
	log    = logutil.New("module", "controller/resource/types")
	debugV = logutil.V(200)
)

func init() {
	depends.Register("resource", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
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

	dep := depends.Runnables{}
	for _, access := range properties {
		dep = append(dep,
			depends.AsRunnable(
				types.Spec{
					Kind: access.InstanceObserver.Plugin.Lookup(),
					Metadata: types.Metadata{
						Name: access.InstanceObserver.Plugin.String(),
					},
				},
			))
	}
	return dep, nil
}

// Properties is the schema of the configuration in the types.Spec.Properties
type Properties map[string]*internal.InstanceAccess

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
