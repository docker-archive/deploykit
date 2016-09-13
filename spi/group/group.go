package group

import (
	"encoding/json"
	"github.com/docker/libmachete/spi/instance"
)

// Plugin defines the functions for a Group plugin.
type Plugin interface {
	WatchGroup(grp Configuration) error

	UnwatchGroup(id ID) error

	InspectGroup(id ID) (Description, error)

	DescribeUpdate(updated Configuration) (string, error)

	UpdateGroup(updated Configuration) error

	StopUpdate(id ID) error

	DestroyGroup(id ID) error
}

// ID is the unique identifier for a Group.
type ID string

// Configuration is the schema for a Group.  The full schema for a Group is defined by the plugin.
type Configuration struct {
	// ID is the unique identifier for the group.
	ID ID

	// Role designates the type of group, which may alter how the group is managed.  The behavior of different
	// group roles is defined by the plugin.
	Role string

	// Properties is the configuration for the group.
	Properties json.RawMessage
}

// Description is a placeholder for the reported state of a Group.
type Description struct {
	Instances []instance.Description
}
