package types

import (
	"context"
	"fmt"

	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log     = logutil.New("module", "controller/pool/types")
	debugV  = logutil.V(500)
	debugV2 = logutil.V(1000)
)

func init() {
	depends.Register("pool", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
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
	dep = append(dep, depends.AsRunnable(
		types.Spec{
			Kind: properties.InstanceObserver.Name.Lookup(),
			Metadata: types.Metadata{
				Name: properties.InstanceObserver.Name.String(),
			},
		},
	))

	return dep, nil
}

// Properties is the schema of the configuration in the types.Spec.Properties
type Properties struct {
	internal.InstanceAccess `json:",inline" yaml:",inline"`

	// Parallelism specifies how many instances are created in parallel
	Parallelism int

	// Count is how many instances of the resource to provision
	Count int
}

// ModelProperties contain fsm tuning parameters
type ModelProperties struct {
	TickUnit            types.Duration
	WaitBeforeProvision fsm.Tick
	WaitBeforeDestroy   fsm.Tick
	ChannelBufferSize   int

	// FSM tuning options
	fsm.Options `json:",inline" yaml:",inline"`
}

// Validate validates the input properties
func (p Properties) Validate(ctx context.Context) error {
	return nil
}

// Options is the controller options that is used at start up of the process.  It's one-time
type Options struct {
	// for overriding globally
	*internal.InstanceObserver `json:",inline" yaml:",inline"`

	// PluginRetryInterval is the interval for retrying to connect to the plugins
	PluginRetryInterval types.Duration

	// MinChannelBufferSize is the min size of the buffered chanels
	MinChannelBufferSize int

	// MinWaitBeforeProvision is the number of ticks before provision of missing resources begins
	MinWaitBeforeProvision fsm.Tick

	// ModelProperties are the config parameters for the workflow model
	ModelProperties `json:",inline" yaml:",inline"`

	// ProvisionDeadline is the deadline for synchronously calling the plugin to provision
	ProvisionDeadline types.Duration

	// DestroyDeadline is the deadline for synchronously calling the plugin to destroy
	DestroyDeadline types.Duration
}

// Validate validates the controller's options
func (p Options) Validate(ctx context.Context) error {
	if p.MinChannelBufferSize == 0 {
		return fmt.Errorf("min channel buffer size cannot be 0")
	}
	if p.MinWaitBeforeProvision == 0 {
		return fmt.Errorf("min wait before provision cannot be 0")
	}
	if p.ChannelBufferSize < p.MinChannelBufferSize {
		return fmt.Errorf("channel buffer size can't be less than %v", p.MinChannelBufferSize)
	}
	if p.WaitBeforeProvision < p.MinWaitBeforeProvision {
		return fmt.Errorf("wait before provision can't be less than %v", p.MinWaitBeforeProvision)
	}
	if p.ProvisionDeadline.Duration() == 0 {
		return fmt.Errorf("bad provision deadline: %v", p.ProvisionDeadline)
	}
	if p.DestroyDeadline.Duration() == 0 {
		return fmt.Errorf("bad destroy deadline: %v", p.DestroyDeadline)
	}
	return nil
}
