package rpc

import (
	"fmt"
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
)

// ImplementsRequest is the rpc wrapper for the Implements method args.
type ImplementsRequest struct {
}

// ImplementsResponse is the rpc wrapper for the Implements return value.
type ImplementsResponse struct {
	APIs []spi.InterfaceSpec
}

// TypesRequest is the request for getting the types exposed by the rpc object
type TypesRequest struct {
}

// InterfaceSpec is the string version of the InterfaceSpec
type InterfaceSpec string

// TypesResponse is the response for returning the types exposed by the rpc object
type TypesResponse struct {
	Types map[InterfaceSpec][]string
}

// Handshake is a simple RPC object for doing handshake
type Handshake map[spi.InterfaceSpec][]string

// Implements responds to a request for the supported plugin interfaces.
func (h Handshake) Implements(_ *http.Request, req *ImplementsRequest, resp *ImplementsResponse) error {
	spi := []spi.InterfaceSpec{}
	for k := range h {
		spi = append(spi, k)
	}
	resp.APIs = spi
	return nil
}

// Types returns a list of types exposed by this object
func (h Handshake) Types(_ *http.Request, req *TypesRequest, resp *TypesResponse) error {
	m := map[InterfaceSpec][]string{}
	for k, v := range h {
		m[InterfaceSpec(fmt.Sprintf("%s/%s", k.Name, k.Version))] = v
	}
	resp.Types = m
	return nil
}

// Handshaker is the interface implemented by all rpc objects that can give information
// about the interfaces it implements
type Handshaker interface {
	// Implementes returns a list of interface specs implemented by this object
	Implements() ([]spi.InterfaceSpec, error)

	// Types returns a list of types exposed by this object
	Types() (map[InterfaceSpec][]string, error)
}
