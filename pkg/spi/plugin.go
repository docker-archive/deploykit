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
