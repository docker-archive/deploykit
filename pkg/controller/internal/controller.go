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

// Managed is the interface implemented by managed objects within a controller
type Managed interface {
	Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error)
	Manage(types.Spec) (*types.Object, error)
	Object() (*types.Object, error)
	Free() (*types.Object, error)
	Dispose() (*types.Object, error)
}

// Controller implements the pkg/controller/Controller interface and manages a collection of controls
type Controller struct {
	alloc   func(types.Spec) (Managed, error)
	keyfunc func(types.Metadata) string
	managed map[string]Managed
	leader  manager.Leadership
	lock    sync.RWMutex
}

// NewController creates a controller injecting dependencies
func NewController(l manager.Leadership,
	alloc func(types.Spec) (Managed, error),
	keyfunc func(types.Metadata) string) *Controller {

	c := &Controller{
		keyfunc: keyfunc,
		alloc:   alloc,
		leader:  l,
		managed: map[string]Managed{},
	}
	return c
}

func (c *Controller) leaderGuard() error {
	is, err := c.leader.IsLeader()
	if err != nil {
		return err
	}
	if !is {
		return fmt.Errorf("not a leader")
	}
	return nil
}

func (c *Controller) getManaged(search *types.Metadata, spec *types.Spec) ([]Managed, error) {
	out := []Managed{}
	if search == nil {
		// all managed objects
		for _, v := range c.managed {
			out = append(out, v)
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
			c.managed[key] = m
		} else {
			return out, nil
		}
	}
	out = append(out, c.managed[key])
	return out, nil
}

// Plan is a commit without actually making the changes.  The controller returns a proposed object state
// after commit, with a Plan, or error.
func (c *Controller) Plan(operation controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	if err = c.leaderGuard(); err != nil {
		return
	}

	m := []Managed{}
	copy := spec
	m, err = c.getManaged(&spec.Metadata, &copy)
	if err != nil {
		return
	}

	// In the future maybe will consider wildcard commits...  but this is highly discouraged at this time.
	if len(m) != 1 {
		err = fmt.Errorf("duplicate objects: %v", m)
	}

	o, p, e := m[0].Plan(operation, spec)
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

	m := []Managed{}
	copy := spec
	m, err = c.getManaged(&spec.Metadata, &copy)
	if err != nil {
		return
	}

	// In the future maybe will consider wildcard commits...  but this is highly discouraged at this time.
	if len(m) != 1 {
		err = fmt.Errorf("duplicate objects: %v", m)
	}

	switch operation {
	case controller.Manage:
		o, e := m[0].Manage(spec)
		if o != nil {
			object = *o
		}
		err = e
		return

	case controller.Destroy:
		o, e := m[0].Dispose()
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

	m := []Managed{}
	m, err = c.getManaged(search, nil)

	log.Debug("Describe", "search", search, "V", debugV, "managed", m, "err", err)

	if err != nil {
		return
	}
	objects = []types.Object{}
	for _, s := range m {
		o, err := s.Object()
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
		m[0].Free()

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
		out[k] = v.(controller.Controller)
	}
	return out, nil
}
