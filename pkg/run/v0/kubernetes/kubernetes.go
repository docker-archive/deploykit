package kubernetes

import (
	"os"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/kubernetes"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "kubernetes"

	// EnvConfigDir is the environment variable to set the config directory
	EnvConfigDir = "INFRAKIT_FLAVOR_KUBERNETES_CONFIG_DIR"
)

var (
	log = logutil.New("module", "run/v0/kubernetes")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

func getWd() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return os.TempDir()
}

func mustBool(b bool, err error) bool {
	if err != nil {
		panic(err)
	}
	return b
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = kubernetes.Options{
	ConfigDir: local.Getenv(EnvConfigDir, getWd()),
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	managerStop := make(chan struct{})
	workerStop := make(chan struct{})

	onStop = func() {
		close(managerStop)
		close(workerStop)
	}

	managerFlavor, e := kubernetes.NewManagerFlavor(scope, options, managerStop)
	if e != nil {
		err = e
		return
	}

	workerFlavor, e := kubernetes.NewWorkerFlavor(scope, options, workerStop)
	if e != nil {
		err = e
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Flavor: map[string]flavor.Plugin{
			"manager": managerFlavor,
			"worker":  workerFlavor,
		},
		run.Metadata: func() (map[string]metadata.Plugin, error) {
			return map[string]metadata.Plugin{
				"manager": managerFlavor,
				"worker":  workerFlavor,
			}, nil
		},
	}
	return
}
