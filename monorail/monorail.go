package monorail

import (
	"github.com/codedellemc/gorackhd/client"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// Monorail type wraps RackHD Monorail client with methods to enable mockable interfaces
type Monorail struct {
	client *client.Monorail
}

// New instantiates a new Monorail client instance
func New(transport runtime.ClientTransport, formats strfmt.Registry) *Monorail {
	client := client.New(transport, formats)
	return &Monorail{client: client}
}

// Nodes provides a RackHD Nodes client
func (m *Monorail) Nodes() NodeIface {
	return m.client.Nodes
}
