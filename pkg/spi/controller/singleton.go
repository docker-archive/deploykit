package controller

import (
	"fmt"

	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

// Singleton returns a singleton that can only run a leader
func Singleton(c Controller, leader func() stack.Leadership) Controller {
	return &singleton{c, leader}
}

// singleton is a controller that only runs on a leader
type singleton struct {
	Controller
	leader func() stack.Leadership
}

func (s *singleton) do(action func() error) error {
	check := s.leader()
	if check == nil {
		return fmt.Errorf("cannot determine leader")
	}

	is, err := check.IsLeader()
	if err != nil {
		return err
	}
	if !is {
		return fmt.Errorf("not a leader")
	}
	return action()
}

// Plan is a commit without actually making the changes.  The controller returns a proposed object state
// after commit, with a Plan, or error.
func (s *singleton) Plan(op Operation, spec types.Spec) (object types.Object, plan Plan, err error) {
	err = s.do(func() error {
		object, plan, err = s.Controller.Plan(op, spec)
		return err
	})
	return
}

// Commit commits the spec to the controller for management or destruction.  The controller's job is to ensure reality
// matches the operation and the specification.  The spec can be composed and references other controllers or plugins.
// When a spec is committed to a controller, the controller returns the object state corresponding to
// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
// the object to be destroyed.
func (s *singleton) Commit(op Operation, spec types.Spec) (object types.Object, err error) {
	err = s.do(func() error {
		object, err = s.Controller.Commit(op, spec)
		return err
	})
	return
}

// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
// A nil Metadata will instruct the controller to return all objects under management.
func (s *singleton) Describe(search *types.Metadata) (objects []types.Object, err error) {
	err = s.do(func() error {
		objects, err = s.Controller.Describe(search)
		return err
	})
	return
}

// Free tells the controller to pause management of objects matching.  To resume, commit again.
func (s *singleton) Free(search *types.Metadata) (objects []types.Object, err error) {
	err = s.do(func() error {
		objects, err = s.Controller.Free(search)
		return err
	})
	return
}
