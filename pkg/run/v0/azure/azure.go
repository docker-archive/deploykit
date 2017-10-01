package azure

import (
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	azure "github.com/docker/infrakit/pkg/provider/azure/plugin"
	azure_instance "github.com/docker/infrakit/pkg/provider/azure/plugin/instance"
	azure_loadbalancer "github.com/docker/infrakit/pkg/provider/azure/plugin/loadbalancer"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "azure"

	// EnvResourceGroup is the env for setting the resource group name
	EnvResourceGroup = "INFRAKIT_AZURE_RESOURCE_GROUP"

	// EnvSubscriptionID is the subscription id
	EnvSubscriptionID = "INFRAKIT_AZURE_SUBSCRIPTION_ID"

	// EnvOAuthToken is the env to set OAuth token
	EnvOAuthToken = "INFRAKIT_AZURE_OAUTH_TOKEN"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_AZURE_NAMESPACE_TAGS"

	// EnvLBNames is the name of the LB ENV variable name for the LB plugin.
	EnvLBNames = "INFRAKIT_AZURE_LB_NAMES"
)

var (
	log = logutil.New("module", "run/v0/azure")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// Options capture the options for starting up the plugin.
type Options struct {
	// Namespace is a set of kv pairs for tags that namespaces the resource instances
	Namespace map[string]string

	// LBNames is a list of names for ELB instances to start the L4 plugins
	LBNames []string

	azure.Options `json:",inline" yaml:",inline"`
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
	LBNames:   strings.Split(local.Getenv(EnvLBNames, ""), ","),
	Options: azure.Options{
		ResourceGroup:  local.Getenv(EnvResourceGroup, ""),
		SubscriptionID: local.Getenv(EnvSubscriptionID, ""),
		Token:          local.Getenv(EnvOAuthToken, ""),
	},
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

	l4Map := map[string]loadbalancer.L4{}
	for _, name := range options.LBNames {
		var lbPlugin loadbalancer.L4
		lbPlugin, e := azure_loadbalancer.NewL4Plugin(name, options.Options)
		if e != nil {
			err = e
			return
		}
		l4Map[name] = lbPlugin
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.L4: func() (map[string]loadbalancer.L4, error) { return l4Map, nil },
		run.Instance: map[string]instance.Plugin{
			"virtualmachine": azure_instance.NewVirtualMachinePlugin(options.Options),
		},
	}

	return
}
