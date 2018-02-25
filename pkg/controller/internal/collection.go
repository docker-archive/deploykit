package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/fsm"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// stateMachine is a struct for a single state machine and its definition
type stateMachine struct {
	fsm.FSM
	*fsm.Spec
}

func (f stateMachine) MarshalJSON() ([]byte, error) {
	state := "-"
	if f.FSM != nil && f.Spec != nil {
		state = f.StateName(f.State())
	}
	return []byte(`"` + state + `"`), nil
}

// Item is an item in the collection.
type Item struct {
	Key   string
	State stateMachine
	Data  map[string]interface{}
}

// Collection is a Managed that tracks a set of finite state machines.
type Collection struct {

	// PlanFunc returns a plan based on the intent
	PlanFunc func(controller.Operation, types.Spec) (controller.Plan, error)

	// StartFunc begins the actual processing. This will be called synchronously
	// so the body needs to start goroutines.
	StartFunc func(context.Context)

	// UpdateSpecFunc is called when a new spec is posted.  This will be executed
	// with exclusive lock on the collection.
	UpdateSpecFunc func(types.Spec) error

	// PauseFunc is called when the controller tries to pause.
	PauseFunc func(bool)

	// StopFunc is called when the collection is stopped terminally.
	StopFunc func() error

	// TerminateFunc is called when this collection is to be destroyed / terminated.
	// This is not the same as Stop, which stops monitoring.
	TerminateFunc func() error

	spec  types.Spec
	items map[string]*Item
	stop  chan struct{}

	scope scope.Scope

	running bool
	freed   bool
	poller  *Poller
	ticker  <-chan time.Time

	metadata        metadata.Plugin
	metadataUpdates chan func(map[string]interface{})

	lock sync.RWMutex
}

// NewCollection returns a Managed controller object that represents a collection
// of finite state machines (FSM).
func NewCollection(scope scope.Scope) (*Collection, error) {
	c := &Collection{
		scope:           scope,
		items:           map[string]*Item{},
		metadataUpdates: make(chan func(map[string]interface{})),
		stop:            make(chan struct{}),
	}

	c.metadata = metadata_plugin.NewPluginFromChannel(c.metadataUpdates)
	return c, nil
}

// Metadata returns a metadata plugin implementation. Optional; ok to be nil
func (c *Collection) Metadata() metadata.Plugin {
	return c.metadata
}

// MetadataRemove removes the object in the metadata plugin interface
func (c *Collection) MetadataRemove(key func(instance.Description) (string, error), v []instance.Description) {
	c.metadataUpdates <- func(view map[string]interface{}) {

		for _, d := range v {

			k, err := key(d)
			if err != nil {
				log.Error("cannot get key", "instance", d)
				continue
			}

			delete(view, k)
		}
	}
}

// MetadataExport exports the objects in the metadata plugin interface. A keyfunc is required to compute
// the key based on the instance.
func (c *Collection) MetadataExport(key func(instance.Description) (string, error), v []instance.Description) error {

	// metadata entry struct ==> this struct copies the instance.Description
	type entry struct {
		ID         instance.ID
		LogicalID  *instance.LogicalID
		Tags       map[string]string
		Properties interface{} // changed from types.Any
	}

	// A single update sets all of the instances
	c.metadataUpdates <- func(view map[string]interface{}) {

		for _, d := range v {

			k, err := key(d)
			if err != nil {
				log.Error("cannot get key", "instance", d)
				continue
			}

			var p interface{}
			if err := d.Properties.Decode(&p); err != nil {
				log.Error("cannot decode properties", "instance", d.ID, "err", err)
			}

			view[k] = &entry{
				ID:         d.ID,
				LogicalID:  d.LogicalID,
				Tags:       d.Tags,
				Properties: p,
			}

		}
	}
	return nil
}

// Put puts an item by key
func (c *Collection) Put(k string, fsm fsm.FSM, spec *fsm.Spec, data map[string]interface{}) *Item {
	if data == nil {
		data = map[string]interface{}{}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if item, has := c.items[k]; has {
		item.Key = k
		item.State = stateMachine{fsm, spec}
		for k, v := range data {
			item.Data[k] = v
		}
	} else {
		c.items[k] = &Item{
			Key:   k,
			State: stateMachine{fsm, spec},
			Data:  data,
		}
	}
	return c.items[k]
}

// Get returns an item by key.
func (c *Collection) Get(k string) *Item {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.items[k]
}

// Delete an item by key
func (c *Collection) Delete(k string) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	delete(c.items, k)
}

// Scope returns the scope the collection uses to access plugins
func (c *Collection) Scope() scope.Scope {
	return c.scope
}

// object returns the state
func (c *Collection) object() (*types.Object, error) {
	snapshot, err := c.snapshot()
	if err != nil {
		return nil, err
	}

	c.spec.Metadata.Identity = &types.Identity{
		ID: c.spec.Metadata.Name,
	}

	object := types.Object{
		Spec:  c.spec,
		State: snapshot,
	}
	return &object, nil
}

// Start starts the managed
func (c *Collection) Start() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.start()
}

func (c *Collection) start() {
	if c.StartFunc == nil {
		return
	}

	log.Debug("starting collection", "V", debugV)

	ctx := context.Background()
	c.StartFunc(ctx)

	c.running = true
}

// Running returns true if managed is running
func (c *Collection) Running() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.running
}

// Stop stops the collection from monitoring and any processing.  This operation is terminal.
func (c *Collection) Stop() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	log.Debug("Stop", "V", debugV)

	if c.metadataUpdates != nil {
		close(c.metadataUpdates)
		c.metadataUpdates = nil
	}

	if c.StopFunc == nil {
		return nil
	}

	return c.StopFunc()
}

// Plan returns a plan, the current state, or error
func (c *Collection) Plan(v controller.Operation, s types.Spec) (*types.Object, *controller.Plan, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	log.Debug("Plan", "op", v, "spec", s, "V", debugV)

	o, err := c.object()
	if err != nil {
		return nil, nil, err
	}

	if c.PlanFunc == nil {
		return o, nil, err
	}

	p, err := c.PlanFunc(v, s)
	return o, &p, err
}

// Enforce will call the behavior to update the spec once it passes validation, and the collection
// will start running / polling.  Since the collection is one-time use (it gets created and replaced by
// the base controller implementation), enforce will be called only once.
func (c *Collection) Enforce(spec types.Spec) (*types.Object, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	log.Debug("Enforce", "spec", spec, "V", debugV)

	if c.UpdateSpecFunc != nil {
		if err := c.UpdateSpecFunc(spec); err != nil {
			return nil, err
		}
	}
	c.freed = false
	c.spec = spec
	c.items = map[string]*Item{} // reset

	c.start()
	return c.object()
}

// Inspect inspects the current state of the collection.
func (c *Collection) Inspect() (*types.Object, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	v, err := c.object()
	log.Debug("Inspect", "object", *v, "err", err, "V", debugV)

	return v, err
}

// Pause pauses the collection from monitoring and reconciling. This is temporary compared to Stop.
func (c *Collection) Pause() (*types.Object, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.PauseFunc == nil {
		return c.Inspect()
	}

	c.PauseFunc(true)
	return c.Inspect()
}

// Free is an alias for Pause
func (c *Collection) Free() (*types.Object, error) {
	return c.Pause()
}

// Terminate destroys the resources associated with this collection.
func (c *Collection) Terminate() (*types.Object, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.TerminateFunc != nil {
		err := c.TerminateFunc()
		if err != nil {
			return nil, err
		}
		return c.Inspect()
	}
	return nil, fmt.Errorf("not supported")
}

func (c *Collection) snapshot() (*types.Any, error) {
	view := []Item{}

	for _, item := range c.items {
		obj := *item
		view = append(view, obj)
	}

	return types.AnyValue(view)
}

// Visit visits the items managed in this collection
func (c *Collection) Visit(v func(Item) bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, item := range c.items {
		if !v(*item) {
			break
		}
	}
}
