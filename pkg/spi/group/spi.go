package group

import (
	"encoding/json"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// InterfaceSpec is the current name and version of the Group API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Group",
	Version: "0.1.0",
}

// Plugin defines the functions for a Group plugin.
type Plugin interface {
	CommitGroup(grp Spec, pretend bool) (string, error)

	FreeGroup(id ID) error

	DescribeGroup(id ID) (Description, error)

	DestroyGroup(id ID) error

	InspectGroups() ([]Spec, error)
}

// ID is the unique identifier for a Group.
type ID string

// Spec is the specification for a Group.  The full schema for a Group is defined by the plugin.
// In general, a Spec of an entity is set as the raw JSON value of another object's Properties.
type Spec struct {
	// ID is the unique identifier for the group.
	ID ID

	// Properties is the configuration for the group.
	// The schema for the raw JSON can be found as the *.Spec of the plugin used.
	// For instance, if the default group plugin is used, the value here will be
	// a JSON representation of github.com/docker/infrakit/plugin/group/types.Spec
	Properties *json.RawMessage
}

// Description is a placeholder for the reported state of a Group.
type Description struct {
	Instances []instance.Description
	Converged bool
}
