package launch

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

// Exec is a service that is able to start plugins based on different
// mechanisms from running local binary to pulling and running docker containers or engine plugins
type Exec interface {

	// Name returns the name of the launcher.  This is used to identify
	// which launcher to use in configurations or command line flags
	Name() string

	// Exec starts the plugin given the name of the plugin and
	// the command and args to start it.
	// This can be an async process but the launcher will poll for the running
	// status of the plugin.
	// The client can receive and block on the returned channel
	// and add optional timeout in its own select statement.
	Exec(kind string, name plugin.Name, config *types.Any) (plugin.Name, <-chan error, error)
}
