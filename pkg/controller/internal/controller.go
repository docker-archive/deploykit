package internal

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Controller implements the pkg/controller/Controller interface and manages a collection of controls
type Controller struct {
	alloc   func(types.Spec) (Managed, error)
	keyfunc func(types.Metadata) string
	managed map[string]*Managed
	events  chan<- *event.Event
	lock    sync.RWMutex
}

// NewController creates a controller injecting dependencies
func NewController(alloc func(types.Spec) (Managed, error),
	keyfunc func(types.Metadata) string) *Controller {

	c := &Controller{
		keyfunc: keyfunc,
		alloc:   alloc,
		managed: map[string]*Managed{},
	}
	return c
}

func (c *Controller) getManaged(search *types.Metadata, spec *types.Spec) ([]**Managed, error) {
	log.Debug("getManaged", "search", search, "spec", spec, "V", debugV)

	out := []**Managed{}
	if search == nil {
		// all managed objects
		for _, v := range c.managed {
			copy := v
			out = append(out, &copy)
		}
		return out, nil
	}

	key := c.keyfunc(*search)
	if key == "" && spec != nil {
		return nil, fmt.Errorf("must specify name")
	}

	if _, has := c.managed[key]; !has {
		if spec != nil {

			m, _, err := c.allocManaged(spec)
			if err != nil {
				return nil, err
			}

			c.managed[key] = &m

		} else {
			return out, nil
		}
	}
	ptr := c.managed[key]
	out = append(out, &ptr)

	log.Debug("found managed", "search", search, "spec", spec, "found", out)
	return out, nil
}

func (c *Controller) allocManaged(spec *types.Spec) (m Managed, pubChan chan *event.Event, err error) {
	m, err = c.alloc(*spec)
	if err != nil {
		return
	}

	if eventPlugin := m.Events(); eventPlugin != nil {
		if publisher, is := eventPlugin.(event.Publisher); is {

			pubChan = make(chan *event.Event)

			go func() {
				// This goroutine will take the events from the Managed (Collection) and
				// forward it to the channel which it was given by the RPC layer.
				for {
					event, ok := <-pubChan
					if !ok {
						return
					}
					c.events <- event
				}
			}()
			publisher.PublishOn(pubChan)
		}
	}

	return
}

// Metadata exposes any metdata implementations
func (c *Controller) Metadata() (plugins map[string]metadata.Plugin, err error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	plugins = map[string]metadata.Plugin{}

	for k, m := range c.managed {
		p := (*m).Metadata()
		if p != nil {
			plugins[k] = p
		}
	}
	return plugins, nil
}

// List implements event.List
func (c *Controller) List(topic types.Path) ([]string, error) {
	return c.dynamicTopics(topic)
}

func (c *Controller) dynamicTopics(topic types.Path) ([]string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if len(topic) == 0 || topic.Dot() {
		out := []string{}
		for k := range c.managed {
			out = append(out, k)
		}
		return out, nil
	}

	key := *topic.Index(0)
	if m, has := c.managed[key]; has {
		if p := (*m).Events(); p != nil {
			return p.List(topic.Shift(1))
		}
	}
	return nil, nil
}

// PublishOn sets the channel to publish on
func (c *Controller) PublishOn(events chan<- *event.Event) {
	c.events = events
}

// Controllers returns a map of managed objects as subcontrollers
func (c *Controller) Controllers() (map[string]controller.Controller, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	out := map[string]controller.Controller{
		"": c,
	}
	for k, v := range c.managed {
		out[k] = &Controller{
			alloc:   c.alloc,
			keyfunc: c.keyfunc,
			managed: map[string]*Managed{
				k: v, // Scope to this as only instance
			},
		}
	}
	return out, nil
}

// Plan is a commit without actually making the changes.  The controller returns a proposed object state
// after commit, with a Plan, or error.
func (c *Controller) Plan(operation controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	m := []**Managed{}
	copy := spec
	m, err = c.getManaged(&spec.Metadata, &copy)
	if err != nil {
		return
	}

	// In the future maybe will consider wildcard commits...  but this is highly discouraged at this time.
	if len(m) != 1 {
		err = fmt.Errorf("duplicate objects: %v", m)
	}

	o, p, e := (**m[0]).Plan(operation, spec)
	if o != nil {
		object = *o
	}
	if p != nil {
		plan = controller.Plan{}
	}
	err = e
	return
}

// Commit commits the spec to the controller for management or destruction.  The controller's job is to ensure reality
// matches the operation and the specification.  The spec can be composed and references other controllers or plugins.
// When a spec is committed to a controller, the controller returns the object state corresponding to
// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
// the object to be destroyed.
func (c *Controller) Commit(operation controller.Operation, spec types.Spec) (object types.Object, err error) {

	c.lock.Lock()
	defer c.lock.Unlock()

	log.Debug("committing", "operation", operation, "spec", spec)

	m := []**Managed{}
	copy := spec
	m, err = c.getManaged(&spec.Metadata, &copy)
	if err != nil {
		log.Error("err", "err", err)
		return
	}

	log.Debug("got managed", "operation", operation, "spec", spec, "m", m)

	if len(m) == 0 {
		return types.Object{}, fmt.Errorf("no managed object found %v", spec.Metadata.Name)
	}

	// In the future maybe will consider wildcard commits...  but this is highly discouraged at this time.
	if len(m) != 1 {
		err = fmt.Errorf("duplicate objects: %v", m)
	}

	switch operation {
	case controller.Enforce:

		managed := *(m[0])
		if (*managed).Running() {

			log.Debug("creating new object to replace running instance.")

			// Create a new object, update default identity
			if spec.Metadata.Identity == nil {
				spec.Metadata.Identity = &types.Identity{
					ID: spec.Metadata.Name,
				}
			}

			// Tell the old to stop
			(*managed).Stop()

			log.Debug("Currently running managed object", "managed", m[0])

			newManaged, _, err := c.allocManaged(&spec)
			if err != nil {
				log.Error("cannot allocate a new managed object", "spec", spec, "err", err)
				return types.Object{}, err
			}

			// continuity of context / spec
			newManaged.SetPrevSpec((*managed).CurrentSpec())

			// swap
			**m[0] = newManaged

			log.Debug("Swapped running managed object", "managed", m[0])
		}

		log.Debug("Calling enforce", "spec", spec, "m", managed, "V", debugV2)
		o, e := (*managed).Enforce(spec)
		log.Debug("Called enforce", "spec", spec, "m", managed, "V", debugV2)
		if o != nil {
			object = *o
		}
		err = e
		return

	case controller.Destroy:
		o, e := (**m[0]).Terminate()
		if o != nil {
			object = *o
		}
		err = e
		return
	default:
		err = fmt.Errorf("unknown operation: %v", operation)
		return
	}
}

// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
// A nil Metadata will instruct the controller to return all objects under management.
func (c *Controller) Describe(search *types.Metadata) (objects []types.Object, err error) {
	defer log.Debug("Describe", "search", search, "V", debugV, "err", err)

	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.describe(search)
}

func (c *Controller) describe(search *types.Metadata) (objects []types.Object, err error) {
	m := []**Managed{}
	m, err = c.getManaged(search, nil)
	if err != nil {
		return
	}

	objects = []types.Object{}
	for _, s := range m {
		o, err := (**s).Inspect()
		if err != nil {
			return nil, err
		}
		if o != nil {
			objects = append(objects, *o)
		}
	}
	return
}

// Free tells the controller to pause management of objects matching.  To resume, commit again.
func (c *Controller) Free(search *types.Metadata) (objects []types.Object, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	described, err := c.describe(search)
	if err != nil {
		return nil, err
	}

	objects = []types.Object{}
	for _, candidate := range described {

		m, err := c.getManaged(&candidate.Metadata, nil)
		if err != nil {
			return nil, err
		}

		if len(m) != 1 {
			continue
		}
		(**m[0]).Free()

		objects = append(objects, candidate)
	}
	return
}
