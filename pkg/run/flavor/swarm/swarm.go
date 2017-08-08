package swarm

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/swarm"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin and also key used to locate the plugin in discovery
	CanonicalName = "flavor-swarm"
)

var log = logutil.New("module", "run/flavor/swarm")

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the group controller.
type Options struct {
	template.Options `json:",inline" yaml:",inline"`

	// Name of the plugin
	Name string

	// ManaagerInitScriptTemplate is the URL of the template for manager init script
	// This is overridden by the value provided in the spec.
	ManagerInitScriptTemplate string

	// WorkerInitScriptTemplate is the URL of the template for worker init script
	// This is overridden by the value provided in the spec.
	WorkerInitScriptTemplate string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Options: template.Options{
		MultiPass: true,
	},
	Name: CanonicalName,
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (name plugin.Name, impls map[run.PluginCode]interface{}, onStop func(), err error) {

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

	managerFlavor := swarm.NewManagerFlavor(plugins, swarm.DockerClient, mt, managerStop)
	workerFlavor := swarm.NewWorkerFlavor(plugins, swarm.DockerClient, wt, workerStop)

	name = plugin.Name(options.Name)
	impls = map[run.PluginCode]interface{}{
		run.Flavor: map[string]flavor.Plugin{
			"manager": managerFlavor,
			"worker":  workerFlavor,
		},
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
