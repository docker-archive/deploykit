package cli

import (
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/rpc/server"
)

// EnsureDirExists makes sure the directory where the socket file will be placed exists.
func EnsureDirExists(dir string) {
	os.MkdirAll(dir, 0700)
}

// RunPlugin runs a plugin server, advertising with the provided name for discovery.
// The plugin should conform to the rpc call convention as implemented in the rpc package.
func RunPlugin(name string, plugin server.VersionedInterface) {

	dir := discovery.Dir()
	EnsureDirExists(dir)

	stoppable, err := server.StartPluginAtPath(path.Join(dir, name), plugin)
	if err != nil {
		log.Error(err)
	}
	if stoppable != nil {
		stoppable.AwaitStopped()
	}
}
