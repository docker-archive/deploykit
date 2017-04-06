package application

import (
	"github.com/docker/infrakit/pkg/types"
)

type Type string
type Operation int

const (
	// TypeError is the type to use for sending errors in the transport of the events.
	TypeError = Type("error")

	ADD Operation = iota
	DELETE
	UPDATE
	GET
)

type Message struct {
	Op       Operation
	Resource string
	Data     *types.Any `json:",omitempty"`
}
