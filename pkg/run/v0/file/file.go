package file

import (
	"os"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/file"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin for starting up, etc.
	CanonicalName = "file"

	// EnvOptionsDir is the environment variable to use to set the default value of Options.Dir
	EnvOptionsDir = "INFRAKIT_INSTANCE_FILE_OPTIONS_DIR"
)

var (
	log = logutil.New("module", "run/v0/file")
)

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Dir is the path of the directory to store the files
	Dir string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Dir: run.GetEnv(EnvOptionsDir, os.TempDir()),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: file.NewPlugin(options.Dir),
	}
	return
}
