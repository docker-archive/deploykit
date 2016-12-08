package plugin

// Endpoint is the address of the plugin service
type Endpoint struct {

	// Name is the key used to refer to this plugin in all JSON configs
	Name string

	// Protocol is the transport protocol -- unix, tcp, etc.
	Protocol string

	// Address is the how to connect - socket file, host:port, etc.
	Address string
}
