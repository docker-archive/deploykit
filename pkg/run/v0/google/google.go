package google

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/provider/google/plugin/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/provider/google/plugin/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "google"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_GOOGLE_NAMESPACE_TAGS"

	// EnvProject is the env to set the project
	EnvProject = "INFRAKIT_GOOGLE_PROJECT"

	// EnvZone is the env to set the zone
	EnvZone = "INFRAKIT_GOOGLE_ZONE"
)

var (
	log = logutil.New("module", "run/v0/google")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	// Project is the GCP project
	Project string

	// Zone is the GCP zone
	Zone string
}

func defaultNamespace() map[string]string {
	t := map[string]string{}
	list := local.Getenv(EnvNamespaceTags, "")
	for _, v := range strings.Split(list, ",") {
		p := strings.Split(v, "=")
		if len(p) == 2 {
			t[p[0]] = p[1]
		}
	}
	return t
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	Project:   local.Getenv(EnvProject, ""),
	Zone:      local.Getenv(EnvZone, ""),
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

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Metadata: metadata_plugin.NewGCEMetadataPlugin(options.Project, options.Zone),
		run.Instance: map[string]instance.Plugin{
			"compute": instance_plugin.NewGCEInstancePlugin(options.Project, options.Zone, options.Namespace),
		},
	}
	return
}
