package enrollment

import (
	"fmt"
	"sync"
	"time"

	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

// enroller implements the internal.Managed interface.
// When constructed, it takes a list of instance id, or logical
// names, or a source that can provide this data, and makes sure
// a downstream instance plugin properly reflects this list.
// When there are new entries in the source, the sink's Provision
// will be called.  When source entries disappear, the sink's
// Destroy will be called.
// At the moment, destroy will not invoke an flavor plugin to
// execute some kind of drain.  That functionality, instead,
// could be implemented as a proxied instance plugin (using the
// interceptor pattern).
type enroller struct {
	spec       types.Spec
	properties enrollment.Properties
	options    enrollment.Options

	scope scope.Scope

	poller *internal.Poller
	ticker <-chan time.Time
	lock   sync.RWMutex

	groupPlugin    group.Plugin    // source -- where members are to be enrolled
	instancePlugin instance.Plugin // sink -- where enrollments are made
	running        bool

	// template that we use to render with a source instance.Description to get the link Key
	sourceKeySelectorTemplate *template.Template
	// template that we use to render with an enrollment instance.Description to get the link Key
	enrollmentKeySelectorTemplate *template.Template
	// template used to render the enrollment's Provision propertiesx
	enrollmentPropertiesTemplate *template.Template
}

func newEnroller(scope scope.Scope, options enrollment.Options) (*enroller, error) {
	l := &enroller{
		scope:   scope,
		options: options,
	}
	if err := l.options.Validate(enrollment.PluginInit); err != nil {
		return nil, err
	}
	l.ticker = time.Tick(l.options.SyncInterval.Duration())

	l.poller = internal.Poll(
		// This determines if the action should be taken when time is up
		func() bool {
			return true
		},
		// This does the work
		func() (err error) {
			return l.sync()
		},
		l.ticker)

	return l, nil
}

// CurrentSpec returns the spec this collection is enforcing
func (l *enroller) CurrentSpec() (s types.Spec) {
	return
}

// SetPrevSpec sets the spec that a previous version of the collection was managing.
// This gives the context to the collection so that it is able to remove resources
// that no longer are needed, for example.
func (l *enroller) SetPrevSpec(s types.Spec) {
	return
}

// GetPrevSpec returns the spec the this collection continues from.
func (l *enroller) GetPrevSpec() (s *types.Spec) {
	return nil
}

// Metadata returns an optional metadata.Plugin implementation
func (l *enroller) Metadata() metadata.Plugin {
	return nil
}

// Events returns events plugin implementation. Optional; ok to be nil
func (l *enroller) Events() event.Plugin {
	return nil
}

// object returns the state
func (l *enroller) object() (*types.Object, error) {
	object := types.Object{
		Spec: l.spec,
	}
	// TODO build the current state
	return &object, nil
}

// Plan implements internal.Managed.Plan
func (l *enroller) Plan(operation controller.Operation, spec types.Spec) (*types.Object, *controller.Plan, error) {

	if operation == controller.Destroy {
		o, _ := l.object()
		return o, nil, nil
	}

	if spec.Properties == nil {
		return nil, nil, fmt.Errorf("missing properties")
	}
	properties := enrollment.Properties{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, nil, err
	}

	// TODO - build a plan
	return &types.Object{
		Spec: spec,
	}, &controller.Plan{}, nil

}

func (l *enroller) updateSpec(spec types.Spec) error {

	l.lock.Lock()
	defer l.lock.Unlock()

	if spec.Options != nil {
		// At runtime, the user can provide overrides to the set of
		// Options used at start up of the plugin.
		// Here we use the options the plugin initialized with as a
		// starting point to parse the input.
		options := l.options // a copy
		if err := spec.Options.Decode(&options); err != nil {
			return err
		}
		if err := options.Validate(enrollment.PluginCommit); err != nil {
			return err
		}
		l.options = options
	}

	if spec.Properties != nil {
		properties := enrollment.Properties{}
		if err := spec.Properties.Decode(&properties); err != nil {
			return err
		}
		l.properties = properties
	}

	l.spec = spec
	// set identity
	l.spec.Metadata.Identity = &types.Identity{
		ID: l.spec.Metadata.Name,
	}
	return nil
}

// Enforce implements internal.Managed.Enforce
func (l *enroller) Enforce(spec types.Spec) (*types.Object, error) {
	log.Debug("Enforce", "spec", spec, "V", debugV)

	if err := l.updateSpec(spec); err != nil {
		return nil, err
	}

	l.Start()
	return l.object()
}

// Inspect implements internal.Managed.Inspect
func (l *enroller) Inspect() (*types.Object, error) {
	return l.object()
}

// Free implements internal.Managed.Free
func (l *enroller) Free() (*types.Object, error) {
	return l.Pause()
}

// Pause implements internal.Managed.Pause
func (l *enroller) Pause() (*types.Object, error) {
	if l.Running() {
		l.Stop()
	}
	return l.Inspect()
}

// Terminate implements internal.Managed.Terminate
func (l *enroller) Terminate() (*types.Object, error) {
	o, err := l.object()
	if err != nil {
		return nil, err
	}

	if l.Running() {
		l.Stop()
	}

	if l.options.DestroyOnTerminate {
		if err := l.destroy(); err != nil {
			// TODO - how do we handle rollback?
			// For now let's not try to restore deleted entries, because
			// there are no guarantees that the restore operations will succeed.
			return o, err
		}
	}
	return o, nil
}

// Start implements internal/ControlLoop.Start
func (l *enroller) Start() {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.poller != nil {
		go l.poller.Run(context.Background())
		l.running = true
	}
}

// Start implements internal/ControlLoop.Stop
func (l *enroller) Stop() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.poller != nil {
		l.poller.Stop()
	}
	return nil
}

// Running implements internal/ControlLoop.Running
func (l *enroller) Running() bool {
	return l.started()
}
