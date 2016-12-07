package spi

// InterfaceSpec is metadata about an API.
type InterfaceSpec struct {
	// Name of the interface.
	Name string

	// Version is the identifier for the API version.
	Version string
}

// Plugin is an interface that all plugins must support.
type Plugin interface {
	// Implements returns the APIs supported by this plugin.
	Implements() ([]InterfaceSpec, error)
}

var specs = map[string]InterfaceSpec{}

// RegisterInterface registers a known, supported Infrakit plugin type and the current interface name/version.
func RegisterInterface(spec InterfaceSpec) {
	specs[spec.Name] = spec
}

// GetInterface returns the supported InterfaceSpec given a known name (e.g. "Instance", "Flavor", "Group")
func GetInterface(name string) InterfaceSpec {
	return specs[name]
}
