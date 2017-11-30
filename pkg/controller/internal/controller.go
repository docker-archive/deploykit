package internal

import (
	"fmt"
	"sync"

	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "controller/internal")

const debugV = logutil.V(500)

// ControlLoop gives status and means to stop the object
type ControlLoop interface {
	Start()
	Running() bool
	Stop() error
}

// Managed is the interface implemented by managed objects within a controller
type Managed interface {
	ControlLoop

	Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error)
	Enforce(types.Spec) (*types.Object, error)
	Inspect() (*types.Object, error)
	Free() (*types.Object, error)
	Terminate() (*types.Object, error)
}

// Controller implements the pkg/controller/Controller interface and manages a collection of controls
type Controller struct {
	alloc   func(types.Spec) (Managed, error)
	keyfunc func(types.Metadata) string
	managed map[string]*Managed
	leader  func() manager.Leadership
	lock    sync.RWMutex
}

// NewController creates a controller injecting dependencies
func NewController(l func() manager.Leadership,
	alloc func(types.Spec) (Managed, error),
	keyfunc func(types.Metadata) string) *Controller {

	c := &Controller{
		keyfunc: keyfunc,
		alloc:   alloc,
		leader:  l,
		managed: map[string]*Managed{},
	}
	return c
}

func (c *Controller) leaderGuard() error {
	check := c.leader()
	if check == nil {
		return fmt.Errorf("cannot determine leader status")
	}

	is, err := check.IsLeader()
	if err != nil {
		return err
	}
	if !is {
		return fmt.Errorf("not a leader")
	}
	return nil
}

func (c *Controller) getManaged(search *types.Metadata, spec *types.Spec) ([]**Managed, error) {
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
			m, err := c.alloc(*spec)
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
	return out, nil
}

// Plan is a commit without actually making the changes.  The controller returns a proposed object state
// after commit, with a Plan, or error.
func (c *Controller) Plan(operation controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	if err = c.leaderGuard(); err != nil {
		return
	}

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
	if err = c.leaderGuard(); err != nil {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	log.Debug("committing", "operation", operation, "spec", spec)

	m := []**Managed{}
	copy := spec
	m, err = c.getManaged(&spec.Metadata, &copy)
	if err != nil {
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

			// Create a new object
			newManaged, err := c.alloc(spec)
			if err != nil {
				log.Error("cannot allocate a new managed object", "spec", spec, "err", err)
				return types.Object{}, err
			}

			// Tell the old to stop
			(*managed).Stop()

			log.Debug("Currently running managed object", "managed", m[0])

			// swap
			**m[0] = newManaged

			log.Debug("Swapped running managed object", "managed", m[0])
		}

		log.Debug("calling enforce", "spec", spec, "m", managed)
		o, e := (*managed).Enforce(spec)
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

	c.lock.Lock()
	defer c.lock.Unlock()

	m := []**Managed{}
	m, err = c.getManaged(search, nil)

	log.Debug("Describe", "search", search, "V", debugV, "managed", m, "err", err)

	if err != nil {
		return
	}

	if len(m) == 0 {
		ss := fmt.Sprintf("%v", search)
		if search != nil {
			ss = fmt.Sprintf("%v", *search)
		}
		return nil, fmt.Errorf("no managed object found %v", ss)
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
	if err = c.leaderGuard(); err != nil {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	described, err := c.Describe(search)
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

// ManagedObjects returns a map of managed objects
func (c *Controller) ManagedObjects() (map[string]controller.Controller, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

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
			leader: c.leader,
		}
	}
	return out, nil
}
