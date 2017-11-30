package enrollment

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/controller"
	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
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

	leader  func() manager.Leadership
	plugins func() discovery.Plugins

	poller *controller.Poller
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

func newEnroller(plugins func() discovery.Plugins,
	leader func() manager.Leadership, options enrollment.Options) *enroller {
	l := &enroller{
		leader:  leader,
		plugins: plugins,
		options: options,
	}

	interval := l.options.SyncInterval.Duration()
	if interval == 0 {
		interval = enrollment.DefaultSyncInterval
	}
	l.ticker = time.Tick(interval)

	l.poller = controller.Poll(
		// This determines if the action should be taken when time is up
		func() bool {
			if mustTrue(l.isLeader()) {
				return true
			}
			return false
		},
		// This does the work
		func() (err error) {
			return l.sync()
		},
		l.ticker)

	return l
}

func (l *enroller) isLeader() (is bool, err error) {
	check := l.leader()
	if check == nil {
		err = fmt.Errorf("cannot determine leader status")
		return
	}
	is, err = check.IsLeader()
	return
}

func mustTrue(v bool, e error) bool {
	if e != nil {
		return false
	}
	return v
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
		options := enrollment.Options{}
		if err := spec.Options.Decode(&options); err != nil {
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
