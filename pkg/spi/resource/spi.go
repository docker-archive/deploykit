package resource

import (
	"encoding/json"

	"github.com/docker/infrakit/pkg/spi"
)

// InterfaceSpec is the current name and version of the resource API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Resource",
	Version: "0.1.0",
}

// Plugin is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Plugin interface {
	// Validate performs local validation on a provision request.
	Validate(resourceType string, req json.RawMessage) error

	// Provision creates a new resource based on the spec.
	Provision(spec Spec) (*ID, error)

	// Destroy terminates an existing instance.
	Destroy(resourceType string, resource ID) error

	// DescribeResources returns descriptions of all resources of the given type matching all of the provided tags.
	DescribeResources(resourceType string, tags map[string]string) ([]Description, error)
}

// ID is the identifier for a resource.
type ID string

// Description contains details about a resource.
type Description struct {
	ID   ID
	Tags map[string]string
}

// Spec is a specification of a resource to be provisioned.
type Spec struct {
	// Type is the resource type.
	Type string

	// Properties is the opaque resource plugin configuration.
	Properties *json.RawMessage

	// Tags are metadata that describes a resource.
	Tags map[string]string
}
