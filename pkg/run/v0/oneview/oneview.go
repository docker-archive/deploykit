package oneview

import (
	"strconv"
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/oneview"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "cli/x")

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "oneview"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_ONEVIEW_NAMESPACE_TAGS"

	// EnvOVURL is the env for setting the HPE Oneview URL to connect to
	EnvOVURL = "INFRAKIT_ONEVIEW_OVURL"

	// EnvOVUser is the HPE Oneview Username
	EnvOVUser = "INFRAKIT_ONEVIEW_OVUSER"

	// EnvOVPass is the HPE Oneview Password
	EnvOVPass = "INFRAKIT_ONEVIEW_OVPASS"

	// EnvOVApi is the HPE Oneview API Version
	EnvOVApi = "INFRAKIT_ONEVIEW_OVAPI"

	// EnvOVCookie is the HPE Oneview Auth Cookie
	EnvOVCookie = "INFRAKIT_ONEVIEW_OVCOOKIE"
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in HPE OneView
	Namespace map[string]string

	// OneViews is a list of OneView Servers - each corresponds to config of a plugin instance
	OneViews []oneview.Options
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

func defaultOVOptions() oneview.Options {
	i, err := strconv.Atoi(local.Getenv(EnvOVApi, "200"))
	if err != nil {
		log.Crit("Error setting the OneView API, defaulting to 200", err.Error)
		i = 200
	}
	return oneview.Options{
		OVUrl:    local.Getenv(EnvOVURL, ""),
		OVUser:   local.Getenv(EnvOVUser, ""),
		OVPass:   local.Getenv(EnvOVPass, ""),
		OVCookie: local.Getenv(EnvOVCookie, ""),
		OVApi:    i,
	}
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	OneViews: []oneview.Options{
		defaultOVOptions(),
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

	for _, ov := range options.OneViews {
		bareMetal[ov.OVUrl] = oneview.NewOneViewInstancePlugin(ov)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: bareMetal,
	}
	return
}
