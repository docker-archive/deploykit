package cli

import (
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/discovery"
	"github.com/docker/infrakit/rpc"
)

// RunPlugin runs a plugin server, advertising with the provided name for discovery.
// THe plugin should conform to the rpc call convention as implemented in the rpc package.
func RunPlugin(name string, plugin interface{}) {
	_, stopped, err := rpc.StartPluginAtPath(path.Join(discovery.Dir(), name), plugin)
	if err != nil {
		log.Error(err)
	}

	<-stopped // block until done
}
