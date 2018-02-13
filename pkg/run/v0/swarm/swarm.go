package swarm

import (
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/swarm"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
)

const (
	// Kind is the canonical name of the plugin and also key used to locate the plugin in discovery
	Kind = "swarm"

	// EnvSelfLogicalID sets the self id of this controller. This will avoid
	// the self node to be updated.
	EnvSelfLogicalID = "INFRAKIT_GROUP_SELF_LOGICAL_ID"
)

var log = logutil.New("module", "run/v0/swarm")

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the group controller.
type Options struct {
	template.Options `json:",inline" yaml:",inline"`

	// ManaagerInitScriptTemplate is the URL of the template for manager init script
	// This is overridden by the value provided in the spec.
	ManagerInitScriptTemplate string

	// WorkerInitScriptTemplate is the URL of the template for worker init script
	// This is overridden by the value provided in the spec.
	WorkerInitScriptTemplate string

	// Docker is the connection info for the Docker client
	Docker docker.ConnectInfo
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Options: template.Options{
		MultiPass: true,
	},
	Docker: docker.ConnectInfo{
		Host: "unix:///var/run/docker.sock",
	},
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

	mt, err := getTemplate(options.ManagerInitScriptTemplate, swarm.DefaultManagerInitScriptTemplate, options.Options)
	if err != nil {
		return
	}
	wt, err := getTemplate(options.WorkerInitScriptTemplate, swarm.DefaultWorkerInitScriptTemplate, options.Options)
	if err != nil {
		return
	}

	managerStop := make(chan struct{})
	workerStop := make(chan struct{})

	managerFlavor := swarm.NewManagerFlavor(scope, swarm.DockerClient, mt, managerStop)
	workerFlavor := swarm.NewWorkerFlavor(scope, swarm.DockerClient, wt, workerStop)
	instancePlugin := swarm.NewInstancePlugin(swarm.DockerClient, options.Docker)

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
		run.Group:    swarm.NewGroupPlugin(instancePlugin),
		run.Instance: instancePlugin,
	}
	onStop = func() {
		close(workerStop)
		close(managerStop)
	}
	return
}

func getTemplate(url string, defaultTemplate string, opts template.Options) (t *template.Template, err error) {
	if url == "" {
		t, err = template.NewTemplate("str://"+defaultTemplate, opts)
		return
	}
	t, err = template.NewTemplate(url, opts)
	return
}
