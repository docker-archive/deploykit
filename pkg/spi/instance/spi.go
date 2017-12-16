package instance

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// LogicalIDTag is the name of the tag that contains the logical ID of the instance
	LogicalIDTag = "infrakit.logical_id"
)

// InterfaceSpec is the current name and version of the Instance API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Instance",
	Version: "0.6.1",
}

var (
	// RollingUpdate is context indicating rolling update
	RollingUpdate = Context{Reason: "rolling_update"}

	// Termination is context indicating termination
	Termination = Context{Reason: "terminate"}
)

// Plugin is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Plugin interface {
	// Validate performs local validation on a provision request.
	Validate(req *types.Any) error

	// Provision creates a new instance based on the spec.
	Provision(spec Spec) (*ID, error)

	// Label labels the instance
	Label(instance ID, labels map[string]string) error

	// Destroy terminates an existing instance.
	Destroy(instance ID, context Context) error

	// DescribeInstances returns descriptions of all instances matching all of the provided tags.
	// The properties flag indicates the client is interested in receiving details about each instance.
	DescribeInstances(labels map[string]string, properties bool) ([]Description, error)
}
