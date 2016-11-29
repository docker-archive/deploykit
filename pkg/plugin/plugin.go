package plugin

import (
	"encoding/json"
)

// EmptyRequest is a fake type created to meet the rpc export requirements
// for GET-type methods requiring no input
type EmptyRequest struct{}

// TypeVersion is the plugin type + version string used by infrakit.  The type and version
// are defined by infrakit and not by the vendor.  The vendor's name and version fields can
// be set in the Info (see Vendor interface).
// For example, Instance/v1.0.0  The string is not case sensitive and
// version should use semantic versioning.
type TypeVersion string

// Meta is metadata for the plugin
type Meta struct {

	// Vendor captures vendor-specific information about this plugin
	Vendor Info

	// Implements is a list of plugin interface and versions this plugin supports
	Implements []TypeVersion

	// Interfaces (optional) is a slice of interface descriptions by the type and version
	Interfaces []Interface `json:",omitempty"`
}

// Interface is a holder for RPC interface version and method descriptions
type Interface struct {
	Name    TypeVersion
	Methods []MethodDescription
}

// MethodDescription contains information about the RPC method such as the request and response
// example structs.  Note that the field names match the JSON-RPC payload spec with fields like
// 'method', 'params'.  The RPC payload can be constructed by taking this structure and add an 'id'
// field for the RPC request id.
type MethodDescription struct {

	// Method is the rpc method to use in the payload field 'method'
	Method string `json:"method"`

	// Params contains example inputs.  This can be a zero value struct or one with defaults
	Params []interface{} `json:"params"`

	// Result contains an example response/ output.  This can be zero value or with defaults
	Result interface{} `json:"result"`
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
