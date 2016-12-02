package spi

// APISpec is metadata about an API.
type APISpec struct {
	// Name of the API.
	Name string

	// Version is the identifier for the API version.
	Version string
}

// Plugin is an interface that all plugins must support.
type Plugin interface {
	// APIs returns the APIs supported by this plugin.
	APIs() ([]APISpec, error)
}
