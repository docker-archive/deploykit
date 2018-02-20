package types

import (
	"context"

	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
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
				Kind: properties.InstanceSource.Plugin.Lookup(),
				Metadata: types.Metadata{
					Name: properties.InstanceSource.Plugin.String(),
				},
			},
		),
		depends.AsRunnable(
			types.Spec{
				Kind: properties.NodeSource.Plugin.Lookup(),
				Metadata: types.Metadata{
					Name: properties.NodeSource.Plugin.String(),
				},
			},
		),
	}, nil
}

// PluginSpec has information about the plugin
type PluginSpec struct {
	// Plugin is the name of the instance plugin
	Plugin plugin.Name

	// Labels are the labels to use when querying for instances. This is the namespace.
	Labels map[string]string

	// Properties is the properties to configure the instance with.
	Properties *types.Any `json:",omitempty" yaml:",omitempty"`
}

// Properties is the schema of the configuration in the types.Spec.Properties
type Properties struct {

	// ObserveInterval is the polling interval for checking nodes and instances
	ObserveInterval types.Duration

	// Model is the workflow model to use
	Model string

	// ModelProperties contains model-specific configurations
	ModelProperties *types.Any

	// InstanceSource is the name of the instance plugin which will receive the
	// synchronization messages of provision / destroy based on the
	// changes in the List
	InstanceSource PluginSpec

	// InstanceKeySelector is a string template for selecting the join key from
	// an instance's instance.Description. This selector template should use escapes
	// so that the template {{ and }} are preserved.  For example,
	// SourceKeySelector: \{\{ .ID \}\}  # selects the ID field.
	InstanceKeySelector string

	// NodeSource is the name of the instance plugin which will get info on the cluster 'nodes' such
	// as swarm engine or k8s kublets.
	NodeSource PluginSpec

	// NodeKeySelector is a string template for selecting the join key from
	// a node's instance.Description.
	NodeKeySelector string
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
