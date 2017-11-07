package stack

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

// InterfaceSpec is the current name and version of the Instance API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Stack",
	Version: "0.1.0",
}

// Interface is a higher-level abstraction for all the groups, controllers, and plugins
type Interface interface {

	// Enforce enforces infrastructure state to match that of the specs.  The set of
	// specs must of for all of the controllers at once.
	Enforce(specs []types.Spec) error

	// Inspect returns the current state of the infrastructure
	Inspect() ([]types.Object, error)

	// Terminate destroys all resources associated with the specs
	Terminate(specs []types.Spec) error
}
