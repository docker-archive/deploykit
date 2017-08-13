package plugin

// Transport describes how the plugin communicates
type Transport struct {
	// Name is the name of the plugin
	Name Name

	// Listen host:port. If not specified, the default behavior is to use unix socket locally
	Listen string

	// Advertise is the host:port used for others to discover this endpoint
	Advertise string

	// Dir is the directory for discovery (ie location of the socket files, etc.)
	// If not specified, it will default to system settings (via environment variable -- see pkg/discovery/local
	Dir string
}

// DefaultTransport returns the default transport based on a simple name.  The default is to
// use unix socket, at directory specified by the pkg/discovery/local discovery mechanism.
func DefaultTransport(name string) Transport {
	return Transport{
		Name: Name(name),
	}
}
