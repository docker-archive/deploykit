package plugin

// Transport describes how the plugin communicates
type Transport struct {
	// Name is the name of the plugin
	Name Name

	// Listen host:port. If not specified, the default behavior is to use unix socket locally
	Listen string

	// Advertise is the host:port used for others to discover this endpoint
	Advertise string
}
