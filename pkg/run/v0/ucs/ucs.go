package ucs

import (
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/ucs"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "cli/x")

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "ucs"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_UCS_NAMESPACE_TAGS"

	// EnvUCSURL is the env for setting the Cisco UCS URL to connect to
	EnvUCSURL = "INFRAKIT_UCS_URL"

	// EnvUCSUser is the Cisco UCS Username
	EnvUCSUser = "INFRAKIT_UCS_USER"

	// EnvUCSPass is the Cisco UCS Password
	EnvUCSPass = "INFRAKIT_UCS_PASS"

	// EnvUCSCookie is the Cisco UCS Auth Cookie
	EnvUCSCookie = "INFRAKIT_UCS_UCSCOOKIE"
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	// TODO - this is currently implemented in AWS and other cloud providers but not
	// in UCS
	Namespace map[string]string

	// UCSs is a collection of UCS Domains - each corresponds to config of a plugin instance
	UCSs []ucs.Options
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

func defaultUCSOptions() ucs.Options {
	return ucs.Options{
		UCSUrl:    local.Getenv(EnvUCSURL, "https://username:password@VCaddress/sdk"),
		UCSUser:   local.Getenv(EnvUCSUser, ""),
		UCSPass:   local.Getenv(EnvUCSPass, ""),
		UCSCookie: local.Getenv(EnvUCSCookie, ""),
	}
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Namespace: defaultNamespace(),
	UCSs: []ucs.Options{
		defaultUCSOptions(),
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

	for _, ucsDomain := range options.UCSs {
		bareMetal[ucsDomain.UCSUrl] = ucs.NewUCSInstancePlugin(ucsDomain)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: bareMetal,
	}
	return
}
