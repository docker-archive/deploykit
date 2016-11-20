package launch

// Launcher is a service that is able to start plugins based on different
// mechanisms from running local binary to pulling and running docker containers or engine plugins
type Launcher interface {

	// Launch starts the plugin.  This can be an async process but the launcher will poll
	// for the running status of the plugin.  The client can receive and block on the returned channel
	// and add optional timeout in its own select statement.
	Launch(cmd string, args ...string) (<-chan error, error)
}
