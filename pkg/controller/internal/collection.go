package internal

import (
	"context"
	"fmt"
	"path"
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

const (
	// SpecHash is the label name used to include a hash of the instance.Spec used to provision
	SpecHash = "infrakit_spec_hash"
	// InstanceLabel is the label name used for labeling the resource with the name in the collection
	InstanceLabel = "infrakit_instance"
	// CollectionLabel is the the label used to label the name of the collection
	CollectionLabel = "infrakit_collection"
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
	Key     string
	Ordinal int
	State   stateMachine
	Data    map[string]interface{} `json:",omitempty"`
}

// Error associates an error
func (i *Item) Error(err error) {
	if i.Data == nil {
		i.Data = map[string]interface{}{}
	}
	i.Data["error"] = err.Error()
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
	UpdateSpecFunc func(cur types.Spec, prev *types.Spec) error `json:"-"`

	// PauseFunc is called when the controller tries to pause.
	PauseFunc func(bool) `json:"-"`

	// StopFunc is called when the collection is stopped terminally.
	StopFunc func() error `json:"-"`

	// TerminateFunc is called when this collection is to be destroyed / terminated.
	// This is not the same as Stop, which stops monitoring.
	TerminateFunc func() error `json:"-"`

	types.Spec

	previous *types.Spec

	items     map[string]*Item
	itemsLock sync.RWMutex

	stop chan struct{}

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

// Topic returns a topic suitable for the events in this collection
func (c *Collection) Topic(p types.Path) types.Path {
	return types.PathFromString(c.Spec.Metadata.Name).Join(p)
}

// EventID is the type of the events emitted by this object
func (c *Collection) EventID(v string) string {
	return path.Join(c.Spec.Metadata.Name, v)
}

// NewCollection returns a Managed controller object that represents a collection
// of finite state machines (FSM).
func NewCollection(scope scope.Scope, topics ...types.Path) (*Collection, error) {
	c := &Collection{
		scope:           scope,
		items:           map[string]*Item{},
		metadataUpdates: make(chan func(map[string]interface{})),
		stop:            make(chan struct{}),
		topics:          map[string]interface{}{},
		events:          make(chan *event.Event),
	}

	stub := func() interface{} { return "TODO" } // TODO - rationalize this

	for _, topic := range append([]types.Path{
		TopicMetadataUpdate,
		TopicMetadataGone,
		TopicCollectionUpdate,
		TopicCollectionGone,
	}, topics...) {
		types.Put(topic, stub, c.topics)
	}
	c.metadata = metadata_plugin.NewPluginFromChannel(c.metadataUpdates)
	return c, nil
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
	go func() {
		for {
			evt, ok := <-c.events
			if !ok {
				close(events)
				return
			}
			events <- evt
			log.Debug("Event", "event", evt, "ok", ok, "V", debugV2)
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

			types.Put(types.PathFromString(k), map[string]interface{}{
				"gone": time.Now(),
			}, view)
		}
	}

	for _, d := range v {

		k, err := key(d)
		if err != nil {
			log.Error("cannot get key", "instance", d)
			continue
		}

		c.events <- event.Event{
			Topic:   c.Topic(TopicMetadataGone),
			Type:    event.Type("MetadataGone"),
			ID:      c.EventID(k),
			Message: "metadata gone",
		}.Init().WithDataMust(k)
	}
}

// MetadataExport exports the objects in the metadata plugin interface. A keyfunc is required to compute
// the key based on the instance.
func (c *Collection) MetadataExport(key func(instance.Description) (string, error), v []instance.Description) error {

	// metadata entry struct ==> this struct copies the instance.Description
	type entry struct {
		ID          instance.ID
		LogicalID   *instance.LogicalID
		Tags        map[string]string
		Properties  interface{} // changed from types.Any
		description instance.Description
	}

	type update struct {
		description instance.Description
		key         string
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

			types.Put(types.PathFromString(k), entry{
				ID:          d.ID,
				LogicalID:   d.LogicalID,
				Tags:        d.Tags,
				Properties:  p,
				description: d,
			}, view)

			c.events <- event.Event{
				Topic:   c.Topic(TopicMetadataUpdate),
				Type:    event.Type("MetadataUpdate"),
				ID:      c.EventID(k),
				Message: "update metadata",
			}.Init().WithDataMust(d)
		}
	}

	return nil
}

// Put puts an item by key - this is unsynchronized so caller / user needs to synchronize the Put
func (c *Collection) Put(k string, fsm fsm.FSM, spec *fsm.Spec, data map[string]interface{}) *Item {

	c.itemsLock.Lock()
	defer c.itemsLock.Unlock()

	changed := false

	defer func() {
		if changed {
			c.events <- event.Event{
				Topic:   c.Topic(TopicCollectionUpdate),
				Type:    event.Type("CollectionUpdate"),
				ID:      c.EventID(k),
				Message: "update collection",
			}.Init().WithDataMust(spec.StateName(fsm.State()))
		}
	}()

	if data == nil {
		data = map[string]interface{}{}
	}

	if item, has := c.items[k]; has {

		changed = item.State.State() != fsm.State()

		item.Key = k
		item.State = stateMachine{fsm, spec}
		for k, v := range data {
			item.Data[k] = v
		}

	} else {

		changed = true

		c.items[k] = &Item{
			Key:   k,
			State: stateMachine{fsm, spec},
			Data:  data,
		}
	}
	return c.items[k]
}

// CurrentSpec returns the spec this collection is enforcing
func (c *Collection) CurrentSpec() (s types.Spec) {
	c.readTxn(func() error {
		s = c.Spec
		return nil
	})
	return
}

// SetPrevSpec sets the spec that a previous version of the collection was managing.
// This gives the context to the collection so that it is able to remove resources
// that no longer are needed, for example.
func (c *Collection) SetPrevSpec(s types.Spec) {
	c.writeTxn(func() error {
		copy := s
		c.previous = &copy
		return nil
	})
}

// GetPrevSpec returns the spec the this collection continues from.
func (c *Collection) GetPrevSpec() (s *types.Spec) {
	c.readTxn(func() error {
		s = c.previous
		return nil
	})
	return
}

// Get returns an item by key. This is unsynchronized so caller / user needs to synchronize as needed.
func (c *Collection) Get(k string) *Item {
	c.itemsLock.RLock()
	defer c.itemsLock.RUnlock()

	return c.items[k]
}

// GetCountByState returns the number of instances in the given state
func (c *Collection) GetCountByState(state fsm.Index) (count int) {
	c.Visit(func(i Item) bool {
		if i.State.State() == state {
			count++
		}
		return true
	})
	return
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
	c.itemsLock.Lock()
	defer func() {
		c.itemsLock.Unlock()
		c.events <- &event.Event{
			Topic:   c.Topic(TopicCollectionGone),
			Type:    event.Type("CollectionGone"),
			ID:      c.EventID(k),
			Message: "Removed from collection",
		}
	}()
	delete(c.items, k)
}

// Scope returns the scope the collection uses to access plugins
func (c *Collection) Scope() scope.Scope {
	return c.scope
}

// object returns the state
func (c *Collection) object() (object *types.Object, err error) {
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

func (c *Collection) writeTxn(txn func() error) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return txn()
}

func (c *Collection) readTxn(txn func() error) error {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return txn()
}

// Plan returns a plan, the current state, or error
func (c *Collection) Plan(op controller.Operation,
	spec types.Spec) (object *types.Object, plan *controller.Plan, err error) {

	err = c.readTxn(func() error {

		log.Debug("Plan", "op", op, "spec", spec, "V", debugV)

		object, err = c.object()
		if err != nil {
			return err
		}

		if c.PlanFunc == nil {
			return nil
		}

		if p, e := c.PlanFunc(op, spec); e == nil {
			plan = &p
		} else {
			err = e
		}

		return err
	})
	return
}

// Enforce will call the behavior to update the spec once it passes validation, and the collection
// will start running / polling.  Since the collection is one-time use (it gets created and replaced by
// the base controller implementation), enforce will be called only once.
func (c *Collection) Enforce(spec types.Spec) (object *types.Object, err error) {

	if err := c.writeTxn(func() error {

		if c.UpdateSpecFunc != nil {
			if err := c.UpdateSpecFunc(spec, c.previous); err != nil {
				log.Error("updating spec", "err", err)
				return err
			}
		}
		c.freed = false
		c.Spec = spec
		c.items = map[string]*Item{} // reset

		c.start()
		log.Debug("started", "V", debugV2)
		return nil

	}); err != nil {
		return nil, err
	}

	err = c.readTxn(func() error {

		object, err = c.object()
		log.Debug("Called enforce", "spec", spec, "object", object, "V", debugV)

		return err
	})

	return
}

// Inspect inspects the current state of the collection.
func (c *Collection) Inspect() (object *types.Object, err error) {
	err = c.readTxn(func() error {
		object, err = c.object()
		log.Debug("Inspect", "object", object, "err", err, "V", debugV)
		return err
	})
	return
}

// Pause pauses the collection from monitoring and reconciling. This is temporary compared to Stop.
func (c *Collection) Pause() (object *types.Object, err error) {
	err = c.writeTxn(func() error {
		if c.PauseFunc != nil {
			c.PauseFunc(true)
		}
		return nil
	})

	return c.Inspect()
}

// Free is an alias for Pause
func (c *Collection) Free() (*types.Object, error) {
	return c.Pause()
}

// Terminate destroys the resources associated with this collection.
func (c *Collection) Terminate() (object *types.Object, err error) {
	object, err = c.Inspect()
	if err != nil {
		return
	}
	err = c.writeTxn(func() error {
		if c.TerminateFunc != nil {
			return c.TerminateFunc()
		}
		return fmt.Errorf("not supported")
	})
	return
}

func (c *Collection) snapshot() (*types.Any, error) {

	c.itemsLock.RLock()
	defer c.itemsLock.RUnlock()

	view := []Item{}

	for _, item := range c.items {
		obj := *item
		view = append(view, obj)
	}

	return types.AnyValue(view)
}

// Visit visits the items managed in this collection.
func (c *Collection) Visit(v func(Item) bool) {

	c.itemsLock.RLock()
	defer c.itemsLock.RUnlock()

	for _, item := range c.items {
		if !v(*item) {
			break
		}
	}
}
