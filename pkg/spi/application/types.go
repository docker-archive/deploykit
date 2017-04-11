package application

import (
	"github.com/docker/infrakit/pkg/types"
)

// Type is the type of an application.  This gives hint about what struct types to map to, etc.
// It also marks one instance of an application as of different nature from another.
type Type string

//Operation : update operation code
type Operation int

const (
	// TypeError is the type to use for sending errors in the transport of the events.
	TypeError = Type("error")

	// ADD new resources
	ADD Operation = iota
	// DELETE resources
	DELETE
	// UPDATE resources
	UPDATE
	// GET resources
	GET
)

// Message :update message struct
type Message struct {
	Op       Operation
	Resource string
	Data     *types.Any `json:",omitempty"`
}
