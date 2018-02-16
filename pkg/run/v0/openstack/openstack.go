package oneview

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/openstack"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "cli/x")

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "openstack"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_OPENSTACK_NAMESPACE_TAGS"

	// EnvOStackURL is the env for setting the OpenStack auth URL to connect to
	EnvOStackURL = "INFRAKIT_OPENSTACK_AUTHURL"

	// EnvOStackUser is the Open Stack Username
	EnvOStackUser = "INFRAKIT_OPENSTACK_USER"

	// EnvOStackPass is the Open Stack Password
	EnvOStackPass = "INFRAKIT_OPENSTACK_PASS"

	// EnvOStackDomain is the Open Stack API Domain
	EnvOStackDomain = "INFRAKIT_OPENSTACK_DOMAIN"

	// EnvOStackProject is the Open Stack Project
	EnvOStackProject = "INFRAKIT_OPENSTACK_PROJECT"
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in Open Stack
	Namespace map[string]string

	// OneViews is a list of OneView Servers - each corresponds to config of a plugin instance
	OpenStacks []openstack.Options
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

func defaultOSOptions() openstack.Options {

	return openstack.Options{
		OStackAuthURL:    local.Getenv(EnvOStackURL, ""),
		OStackUserName:   local.Getenv(EnvOStackUser, ""),
		OStackPassword:   local.Getenv(EnvOStackPass, ""),
		OStackProject:    local.Getenv(EnvOStackProject, ""),
		OStackUserDomain: local.Getenv(EnvOStackDomain, ""),
	}
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	OpenStacks: []openstack.Options{
		defaultOSOptions(),
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

	bareMetal := map[string]instance.Plugin{}

	for _, os := range options.OpenStacks {
		bareMetal[os.OStackAuthURL] = openstack.NewOpenStackInstancePlugin(os)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: bareMetal,
	}
	return
}
