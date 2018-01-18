package group

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// GroupTag is the name of the tag that contains the group name
	GroupTag = "infrakit.group"
	// ConfigSHATag is the name of the tag that contains the group SHA hash
	ConfigSHATag = "infrakit.config.hash"
)

// InterfaceSpec is the current name and version of the Group API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Group",
	Version: "0.1.1",
}

// AllocationMethod defines the type of allocation and supervision needed by a flavor's Group.
type AllocationMethod struct {
	Size       uint
	LogicalIDs []instance.LogicalID
}

// Index is the index of the instance's creation.  It provides a context for knowing
// what is being created.
type Index struct {

	// Group is the name of the group
	Group ID

	// Sequence is a sequence number that's per instance.
	Sequence uint
}

// Plugin defines the functions for a Group plugin.
type Plugin interface {
	CommitGroup(grp Spec, pretend bool) (string, error)

	FreeGroup(ID) error

	DescribeGroup(ID) (Description, error)

	DestroyGroup(ID) error

	InspectGroups() ([]Spec, error)

	// DestroyInstances deletes instances from this group. Error is returned either on
	// failure or if any instances don't belong to the group. This function
	// should wait until group size is updated.
	DestroyInstances(ID, []instance.ID) error

	// Size returns the current size of the group.
	Size(ID) (int, error)

	// SetSize sets the size.
	// This function should block until completion.
	SetSize(ID, int) error
}

// ID is the unique identifier for a Group.
type ID string

// Spec is the specification for a Group.  The full schema for a Group is defined by the plugin.
// In general, a Spec of an entity is set as the raw JSON value of another object's Properties.
type Spec struct {
	// ID is the unique identifier for the group.
	ID ID

	// Properties is the configuration for the group.
	// The schema for the raw Any can be found as the *.Spec of the plugin used.
	// For instance, if the default group plugin is used, the value here will be
	// an Any / encoded representation of github.com/docker/infrakit/plugin/group/types.Spec
	Properties *types.Any
}

// Description is a placeholder for the reported state of a Group.
type Description struct {
	Instances []instance.Description
	Converged bool
}
