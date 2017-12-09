package ingress

import (
	"github.com/docker/infrakit/pkg/controller/ingress"
	ingress_types "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"

	// load the handlers for ingress con
	_ "github.com/docker/infrakit/pkg/controller/ingress/swarm"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "ingress"

	//EnvOptionsGroup is the env variable to set the backend group controller
	EnvOptionsGroup = "INFRAKIT_INGRESS_GROUP"

	// EnvOptionsBackend is the environment variable to use to set the default value of Options.Backend
	EnvOptionsBackend = "INFRAKIT_INGRESS_BACKEND"
)

var (
	log = logutil.New("module", "run/v0/ingress")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

func leadership(plugins func() discovery.Plugins) stack.Leadership {
	// Scan for a manager
	pm, err := plugins().List()
	if err != nil {
		log.Error("Cannot list plugins", "err", err)
		return nil
	}

	for _, endpoint := range pm {
		rpcClient, err := client.New(endpoint.Address, stack.InterfaceSpec)
		if err == nil {
			return manager_rpc.Adapt(rpcClient)
		}

		if !client.IsErrInterfaceNotSupported(err) {
			log.Error("Got error getting manager", "endpoint", endpoint, "err", err)
			return nil
		}
	}
	return nil
}

// DefaultOptions container options for default behavior
var DefaultOptions = ingress_types.Options{
	SyncInterval: types.FromDuration(ingress_types.DefaultSyncInterval),
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

	log.Info("Decoded input", "config", options)

	transport.Name = name
	impls = map[run.PluginCode]interface{}{

		// TODO - move leadership into scope lookup / stack
		run.Controller: ingress.NewTypedControllers(scope,
			func() stack.Leadership {
				return leadership(scope.Plugins)
			}),
	}

	return
}
