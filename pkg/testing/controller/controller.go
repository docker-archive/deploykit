package controller

import (
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

// Controller implements the controller.Controller interface and supports
// testing by letting user assemble behavior dyanmically.
type Controller struct {
	// Plan is a commit without actually making the changes.  The controller returns a proposed object state
	// after commit, with a Plan, or error.
	DoPlan func(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error)

	// Commit commits the spec to the controller for management.  The controller's job is to ensure reality
	// matches the specification.  The spec can be composed and references other controllers or plugins.
	// When a spec is committed to a controller, the controller returns the object state corresponding to
	// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
	// the object to be destroyed.
	DoCommit func(operation controller.Operation, spec types.Spec) (types.Object, error)

	// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
	// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
	// A nil Metadata will instruct the controller to return all objects under management.
	DoDescribe func(metadata *types.Metadata) ([]types.Object, error)

	// Free tells the controller to pause management of objects matching.  To resume, commit again.
	DoFree func(metadata *types.Metadata) ([]types.Object, error)
}

// Plan implements pkg/controller/Controller.Plan
func (t *Controller) Plan(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error) {
	return t.DoPlan(operation, spec)
}

// Commit implements pkg/controller/Controller.Commit
func (t *Controller) Commit(operation controller.Operation, spec types.Spec) (types.Object, error) {
	return t.DoCommit(operation, spec)
}

// Describe implements pkg/controller/Controller.Describe
func (t *Controller) Describe(metadata *types.Metadata) ([]types.Object, error) {
	return t.DoDescribe(metadata)
}

// Free implements pkg/controller/Controller.Free
func (t *Controller) Free(metadata *types.Metadata) ([]types.Object, error) {
	return t.DoFree(metadata)

}
