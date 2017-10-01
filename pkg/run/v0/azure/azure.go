package azure

import (
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	azure_instance "github.com/docker/infrakit/pkg/provider/azure/plugin/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "azure"

	// EnvResourceGroupName is the env for setting the resource group name
	EnvResourceGroupName = "INFRAKIT_AZURE_RESOURCE_GROUP_NAME"

	// EnvSubscriptionID is the subscription id
	EnvSubscriptionID = "INFRAKIT_AZURE_SUBSCRIPTION_ID"

	// EnvOAuthToken is the env to set OAuth token
	EnvOAuthToken = "INFRAKIT_AZURE_OAUTH_TOKEN"

	// EnvNamespaceTags is the env to set for namespace tags. It's k=v,...
	EnvNamespaceTags = "INFRAKIT_AZURE_NAMESPACE_TAGS"
)

var (
	log = logutil.New("module", "run/v0/azure")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
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
var DefaultOptions = azure_instance.Options{
	Namespace:         defaultNamespace(),
	ResourceGroupName: local.Getenv(EnvResourceGroupName, ""),
	SubscriptionID:    local.Getenv(EnvSubscriptionID, ""),
	Token:             local.Getenv(EnvOAuthToken, ""),
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
	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"virtualmachine": azure_instance.NewVirtualMachinePlugin(options),
		},
	}

	return
}
