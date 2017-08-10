package kubernetes

import (
	"os"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/kubernetes"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin for starting up, etc.
	CanonicalName = "kubernetes"

	// EnvConfigDir is the environment variable to set the config directory
	EnvConfigDir = "INFRAKIT_FLAVOR_KUBERNETES_CONFIG_DIR"
)

var (
	log = logutil.New("module", "run/v0/kubernetes")
)

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// ConfigDir is the path of the directory to store the files
	ConfigDir string

	// ManaagerInitScriptTemplate is the URL of the template for manager init script
	// This is overridden by the value provided in the spec.
	ManagerInitScriptTemplate string

	// WorkerInitScriptTemplate is the URL of the template for worker init script
	// This is overridden by the value provided in the spec.
	WorkerInitScriptTemplate string
}

func getWd() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return os.TempDir()
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	ConfigDir: run.GetEnv(EnvConfigDir, getWd()),
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

	var mt, wt *template.Template

	mt, err = getTemplate(options.ManagerInitScriptTemplate,
		kubernetes.DefaultManagerInitScriptTemplate,
		kubernetes.DefaultTemplateOptions)
	if err != nil {
		return
	}
	wt, err = getTemplate(options.WorkerInitScriptTemplate,
		kubernetes.DefaultWorkerInitScriptTemplate,
		kubernetes.DefaultTemplateOptions)
	if err != nil {
		return
	}

	managerStop := make(chan struct{})
	workerStop := make(chan struct{})

	onStop = func() {
		close(managerStop)
		close(workerStop)
	}

	managerFlavor := kubernetes.NewManagerFlavor(plugins, mt, options.ConfigDir, managerStop)
	workerFlavor := kubernetes.NewWorkerFlavor(plugins, wt, options.ConfigDir, workerStop)

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Flavor: map[string]flavor.Plugin{
			"manager": managerFlavor,
			"worker":  workerFlavor,
		},
		run.Metadata: map[string]metadata.Plugin{
			"manager": managerFlavor,
			"worker":  workerFlavor,
		},
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
