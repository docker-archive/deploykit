package packet

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	packet "github.com/docker/infrakit/pkg/provider/packet/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "packet"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_PACKET_NAMESPACE_TAGS"

	// EnvProject is the env to set the project
	EnvProject = "INFRAKIT_PACKET_PROJECT"

	// EnvAPIToken is the env to set the API token
	EnvAPIToken = "INFRAKIT_PACKET_API_TOKEN"
)

var (
	log = logutil.New("module", "run/v0/packet")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	// Project is the Packet project
	Project string

	// APIToken is the API token
	APIToken string
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
	APIToken:  local.Getenv(EnvAPIToken, ""),
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
		run.Instance: map[string]instance.Plugin{
			"compute": packet.NewPlugin(options.Project, options.APIToken, options.Namespace),
		},
	}
	return
}
