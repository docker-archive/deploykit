package rpc

import (
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
)

// Object is the RPC remote object
type Object struct {
	// Name is the simple name of the object (e.g. aws/ec2-instance ==> ec2-instance).
	Name string

	// ProxyFor is the Name of the plugin for which this object is a proxy (if applicable).
	ProxyFor plugin.Name
}

// Handshake is a simple RPC object for doing handshake
type Handshake map[spi.InterfaceSpec]func() []Object

// Hello returns a list of hello exposed by this object
func (h Handshake) Hello(_ *http.Request, req *HelloRequest, resp *HelloResponse) error {
	resp.Objects = map[string][]Object{}
	for k, v := range h {
		resp.Objects[k.Encode()] = v()
	}
	return nil
}

// HelloRequest is the rpc request for Hello
type HelloRequest struct {
}

// HelloResponse is the rpc response for Hello
type HelloResponse struct {
	// Objects is an index of Objects by Interface
	Objects map[string][]Object
}

// Handshaker is the interface implemented by all rpc objects that can give information
// about the interfaces it implements
type Handshaker interface {

	// Hello is initiated by the client to get some information from the server
	Hello() (map[spi.InterfaceSpec][]Object, error)
}
