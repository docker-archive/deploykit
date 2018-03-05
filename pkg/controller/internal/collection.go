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
	"github.com/docker/infrakit/pkg/spi/event"
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
	Data  map[string]interface{} `json:",omitempty"`
}

// Collection is a Managed that tracks a set of finite state machines.
type Collection struct {

	// PlanFunc returns a plan based on the intent
	PlanFunc func(controller.Operation, types.Spec) (controller.Plan, error) `json:"-"`

	// StartFunc begins the actual processing. This will be called synchronously
	// so the body needs to start goroutines.
	StartFunc func(context.Context) `json:"-"`

	// UpdateSpecFunc is called when a new spec is posted.  This will be executed
	// with exclusive lock on the collection.
	UpdateSpecFunc func(types.Spec) error `json:"-"`

	// PauseFunc is called when the controller tries to pause.
	PauseFunc func(bool) `json:"-"`

	// StopFunc is called when the collection is stopped terminally.
	StopFunc func() error `json:"-"`

	// TerminateFunc is called when this collection is to be destroyed / terminated.
	// This is not the same as Stop, which stops monitoring.
	TerminateFunc func() error `json:"-"`

	types.Spec

	items map[string]*Item // read/writes of this will not be synchronized by the lock.
	stop  chan struct{}

	scope scope.Scope

	running bool
	freed   bool
	poller  *Poller
	ticker  <-chan time.Time

	metadata        metadata.Plugin
	metadataUpdates chan func(map[string]interface{})

	topics map[string]interface{} // events
	events chan *event.Event

	// This lock is used to guard the Managed methods.
	lock sync.RWMutex
}

var (

	// TopicMetadataUpdate is the topic to get metadata updates
	TopicMetadataUpdate = types.PathFromString("metadata/update")

	// TopicMetadataGone is the topic to get metadata gone
	TopicMetadataGone = types.PathFromString("metadata/gone")

	// TopicCollectionUpdate is the topic to get collection updates
	TopicCollectionUpdate = types.PathFromString("collection/update")

	// TopicCollectionGone is the topic to get collection gones
	TopicCollectionGone = types.PathFromString("collection/gone")
)

// NewCollection returns a Managed controller object that represents a collection
// of finite state machines (FSM).
func NewCollection(scope scope.Scope) (*Collection, error) {
	c := &Collection{
		scope:           scope,
		items:           map[string]*Item{},
		metadataUpdates: make(chan func(map[string]interface{})),
		stop:            make(chan struct{}),
		topics:          map[string]interface{}{},
		events:          make(chan *event.Event, 64),
	}

	stub := func() interface{} { return "TODO" } // TODO - rationalize this

	for _, topic := range []types.Path{
		TopicMetadataUpdate,
		TopicMetadataGone,
		TopicCollectionUpdate,
		TopicCollectionGone,
	} {
		types.Put(topic, stub, c.topics)
	}
	c.metadata = metadata_plugin.NewPluginFromChannel(c.metadataUpdates)
	return c, nil
}

// EventType is the type of the events emitted by this object
func (c *Collection) EventType() event.Type {
	return event.Type("resource/" + c.Spec.Metadata.Name)
}

// Metadata returns a metadata plugin implementation. Optional; ok to be nil
func (c *Collection) Metadata() metadata.Plugin {
	return c.metadata
}

// Events returns events plugin implementation. Optional; ok to be nil
func (c *Collection) Events() event.Plugin {
	return c
}

// EventCh returns the events channel to publish events
func (c *Collection) EventCh() chan<- *event.Event {
	return c.events
}

// List implements event.List
func (c *Collection) List(topic types.Path) ([]string, error) {
	return types.List(topic, c.topics), nil
}

// PublishOn sets the channel to publish on
func (c *Collection) PublishOn(events chan<- *event.Event) {
	log.Debug("PublishOn")
	go func() {
		for {
			evt, ok := <-c.events

			log.Debug("Event", "event", evt, "ok", ok, "V", debugV2)

			if !ok {
				return
			}

			// non-blocking send
			select {
			case events <- evt:
			default:
			}
		}
	}()

	return
}

// MetadataGone removes the object in the metadata plugin interface
func (c *Collection) MetadataGone(key func(instance.Description) (string, error), v []instance.Description) {
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

	for _, d := range v {

		k, err := key(d)
		if err != nil {
			log.Error("cannot get key", "instance", d)
			continue
		}

		c.events <- event.Event{
			Type: c.EventType(),
			ID:   k,
		}.Init().Now().WithTopic(TopicMetadataGone.String()).WithDataMust(k)
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

	for _, d := range v {
		c.events <- event.Event{
			Type: c.EventType(),
			ID:   string(d.ID),
		}.Init().Now().WithTopic(TopicMetadataUpdate.String()).WithDataMust(d)
	}

	return nil
}

// Put puts an item by key - this is unsynchronized so caller / user needs to synchronize the Put
func (c *Collection) Put(k string, fsm fsm.FSM, spec *fsm.Spec, data map[string]interface{}) *Item {

	defer func() {
		c.events <- event.Event{
			Type: c.EventType(),
			ID:   string(k),
		}.Init().Now().WithTopic(TopicCollectionUpdate.String()).WithDataMust(spec.StateName(fsm.State()))
	}()

	if data == nil {
		data = map[string]interface{}{}
	}

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

// Get returns an item by key. This is unsynchronized so caller / user needs to synchronize as needed.
func (c *Collection) Get(k string) *Item {
	return c.items[k]
}

// GetByFSM returns an item by the state machine
func (c *Collection) GetByFSM(f fsm.FSM) (item *Item) {
	c.Visit(func(i Item) bool {
		if i.State.ID() == f.ID() {
			copy := i
			item = &copy
			return false
		}
		return true
	})
	return
}

// Delete an item by key. This is unsychronized.
func (c *Collection) Delete(k string) {
	defer func() {
		c.events <- event.Event{
			Type: c.EventType(),
			ID:   string(k),
		}.Init().Now().WithTopic(TopicCollectionGone.String())
	}()
	delete(c.items, k)
}

// Scope returns the scope the collection uses to access plugins
func (c *Collection) Scope() scope.Scope {
	return c.scope
}

// object returns the state
func (c *Collection) object() (object *types.Object, err error) {
	defer log.Debug("object", "ref", object, "err", err)
	snapshot, e := c.snapshot()
	if e != nil {
		err = e
		return
	}

	c.Spec.Metadata.Identity = &types.Identity{
		ID: c.Spec.Metadata.Name,
	}

	object = &types.Object{
		Spec:  c.Spec,
		State: snapshot,
	}

	return
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

	if c.events != nil {
		close(c.events)
		c.events = nil
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

	defer log.Debug("Enforce", "spec", spec, "V", debugV)

	if c.UpdateSpecFunc != nil {
		if err := c.UpdateSpecFunc(spec); err != nil {
			log.Error("updating spec", "err", err)
			return nil, err
		}
	}
	c.freed = false
	c.Spec = spec
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

// Visit visits the items managed in this collection. This is unsynchronized.
func (c *Collection) Visit(v func(Item) bool) {
	for _, item := range c.items {
		if !v(*item) {
			break
		}
	}
}
