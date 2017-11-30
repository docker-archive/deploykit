package enrollment

import (
	enrollment_controller "github.com/docker/infrakit/pkg/controller/enrollment"
	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "enrollment"
)

var (
	log = logutil.New("module", "run/v0/enrollment")
)

func init() {
	inproc.Register(Kind, Run, DefaultOptions)
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = enrollment.Options{
	SyncInterval:       types.Duration(enrollment.DefaultSyncInterval),
	DestroyOnTerminate: false,
}

func leadership(plugins func() discovery.Plugins) manager.Leadership {
	// Scan for a manager
	pm, err := plugins().List()
	if err != nil {
		log.Error("Cannot list plugins", "err", err)
		return nil
	}

	for _, endpoint := range pm {
		rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
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
		run.Controller: enrollment_controller.NewTypedControllers(scope.Plugins,
			func() manager.Leadership {
				return leadership(scope.Plugins)
			}, options),
	}

	return
}
