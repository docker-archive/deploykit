package controller

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

// Plan models the steps the controller will take to fulfill specification when committed.
type Plan struct {
	// Message contains human-friendly message
	Message []string
}

// Operation is the action to be taken for a commit
type Operation int

const (
	// Enforce represents create, update, reconcile operations
	Enforce Operation = iota

	// Destroy is the destroy operation. Destroy also implies Free.
	Destroy
)

var (
	// InterfaceSpec is the current name and version of the Instance API.
	InterfaceSpec = spi.InterfaceSpec{
		Name:    "Controller",
		Version: "0.1.1",
	}
)

// Controller is the interface that all controllers implement.  Controllers are managed by pkg/manager/Manager
type Controller interface {

	// Plan is a commit without actually making the changes.  The controller returns a proposed object state
	// after commit, with a Plan, or error.
	Plan(Operation, types.Spec) (types.Object, Plan, error)

	// Commit commits the spec to the controller for management or destruction.  The controller's job is to ensure reality
	// matches the operation and the specification.  The spec can be composed and references other controllers or plugins.
	// When a spec is committed to a controller, the controller returns the object state corresponding to
	// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
	// the object to be destroyed.
	Commit(Operation, types.Spec) (types.Object, error)

	// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
	// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
	// A nil Metadata will instruct the controller to return all objects under management.
	Describe(*types.Metadata) ([]types.Object, error)

	// Free tells the controller to pause management of objects matching.  To resume, commit again.
	Free(*types.Metadata) ([]types.Object, error)
}
