package cli

import (
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/rpc/server"
)

// RunPlugin runs a plugin server, advertising with the provided name for discovery.
// The plugin should conform to the rpc call convention as implemented in the rpc package.
func RunPlugin(name string, plugin interface{}) {
	stoppable, err := server.StartPluginAtPath(path.Join(discovery.Dir(), name), plugin)
	if err != nil {
		log.Error(err)
	}
	stoppable.AwaitStopped()
}
