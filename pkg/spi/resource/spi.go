package resource

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

// InterfaceSpec is the current name and version of the Resource API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Resource",
	Version: "0.1.1",
}

// ID is the unique identifier for a collection of resources.
type ID string

// Spec is a specification of resources to provision.
type Spec struct {

	// ID is the unique identifier for the collection of resources.
	ID ID

	// Properties is the opaque configuration for the resources.
	Properties *types.Any
}

// Plugin defines the functions for a Resource plugin.
type Plugin interface {
	Commit(spec Spec, pretend bool) (string, error)
	Destroy(spec Spec, pretend bool) (string, error)
	DescribeResources(spec Spec) (string, error)
}
