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

	DestroyGroup(id ID) error
}

// ID is the unique identifier for a Group.
type ID string

// Configuration is the schema for a Group.  The full schema for a Group is defined by the plugin.
type Configuration struct {
	ID         ID
	Properties json.RawMessage
}

// Description is a placeholder for the reported state of a Group.
type Description struct {
	Instances []instance.Description
}
