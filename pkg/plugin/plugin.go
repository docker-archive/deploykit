package plugin

import (
	"encoding/json"

	"github.com/docker/infrakit/pkg/spi"
)

// Informer is the interface that gives information about the plugin such as version and interface methods
type Informer interface {

	// GetMeta returns metadata about the plugin
	GetMeta() (Meta, error)
}

// Meta is metadata for the plugin
type Meta struct {

	// Vendor captures vendor-specific information about this plugin
	Vendor Info

	// Implements is a list of plugin interface and versions this plugin supports
	Implements []spi.InterfaceSpec

	// Interfaces (optional) is a slice of interface descriptions by the type and version
	Interfaces []InterfaceDescription `json:",omitempty"`
}

// InterfaceDescription is a holder for RPC interface version and method descriptions
type InterfaceDescription struct {
	spi.InterfaceSpec
	Methods []MethodDescription
}

// MethodDescription contains information about the RPC method such as the request and response
// example structs.  The request value can be used as an example input, possibly with example
// plugin-custom properties if the underlying plugin implements the InputExample interface.
// The response value gives an example of the example response.
type MethodDescription struct {
	// Request is the RPC request example
	Request Request

	// Response is the RPC response example
	Response Response
}

// Request models the RPC request payload
type Request struct {

	// Version is the version of the JSON RPC protocol
	Version string `json:"jsonrpc"`

	// Method is the rpc method to use in the payload field 'method'
	Method string `json:"method"`

	// Params contains example inputs.  This can be a zero value struct or one with defaults
	Params interface{} `json:"params"`

	// ID is the request is
	ID string `json:"id"`
}

// Response is the RPC response struct
type Response struct {

	// Result is the result of the call
	Result interface{} `json:"result"`

	// ID is id matching the request ID
	ID string `json:"id"`
}

var (
	// NoInfo indicates nothing is known about the plugin
	NoInfo = Info{}
)

// Info is the struct stores the information about the plugin, such as version and parameter types
type Info struct {

	// Name of the plugin.  This is a vendor specific name
	Name string

	// Version of the plugin.  This is a vendor specific version separate from the infrakit
	// api version
	Version string
}

// Vendor is the interface that has vendor-specific information methods
type Vendor interface {

	// Info returns an info struct about the plugin
	Info() Info
}

// InputExample interface is an optional interface implemented by the plugin that will provide
// example input struct to document the vendor-specific api of the plugin. An example of this
// is to provide a sample JSON for all the Properties field in the plugin API.
type InputExample interface {

	// ExampleProperties returns an example JSON raw message that the vendor plugin understands.
	// This is an example of what the user will configure and what will be used as the opaque
	// blob in all the plugin methods where raw JSON messages are referenced.
	ExampleProperties() *json.RawMessage
}

// Endpoint is the address of the plugin service
type Endpoint struct {

	// Name is the key used to refer to this plugin in all JSON configs
	Name string

	// Protocol is the transport protocol -- unix, tcp, etc.
	Protocol string

	// Address is the how to connect - socket file, host:port, etc.
	Address string
}
