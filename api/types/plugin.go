package types

// PluginProperties is an opaque value representing the configuration of a plugin
type PluginProperties interface{}

// Plugin models a plugin or extension element in the system. It can be implemented by a process,
// a in-process routine, or Docker containers/ plugins.
// As a type we do not require the properties to be any form, other than a valid golang value
// which the plugin itself understands.  Different strategies are used for parsing, for example,
// for JSON representations, we use json.RawMessage for the value of the Properties at parsing time.
type Plugin struct {
	Callable `json:"-"`

	Name       string           `json:"plugin"`
	Properties PluginProperties `json:"properties,omitempty"`
}

// Callable injects behavior to a plugin object
type Callable interface {
	// Call makes a call to the plugin using http method, at op (endpoint), with message and result structs
	Call(method, op string, message, result interface{}) ([]byte, error)
}
