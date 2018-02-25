package enrollment

import (
	"strconv"

	"github.com/docker/infrakit/pkg/controller/enrollment"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "enrollment"
)

var (
	// EnvSyncInterval sets the sync interval for all
	// enrollment controller instances in the process
	EnvSyncInterval = "INFRAKIT_ENROLLMENT_SYNC_INTERVAL"

	// EnvDestroyOnTerminate sets the destroyOnTerminate option
	EnvDestroyOnTerminate = "INFRAKIT_ENROLLMENT_DESTROY_ON_TERMINATE"

	log = logutil.New("module", "run/v0/enrollment")

	defaultOptions = enrollment.DefaultOptions
)

func init() {

	// We let the user set some environment variables to override
	// the default values.  These default options are then overridden
	// after the plugin started if the user provides options in the spec
	// to override them.
	if d := types.MustParseDuration(local.Getenv(EnvSyncInterval, "0s")); d > 0 {
		defaultOptions.SyncInterval = d
	}
	if v := local.Getenv(EnvDestroyOnTerminate, ""); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			defaultOptions.DestroyOnTerminate = b
		}
	}

	inproc.Register(Kind, Run, defaultOptions)
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

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := defaultOptions // decode into a copy of the updated defaults
	err = config.Decode(&options)
	if err != nil {
		return
	}

	log.Info("Decoded input", "config", options)

	leader := func() stack.Leadership {
		return leadership(scope.Plugins)
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Controller: func() (map[string]controller.Controller, error) {
			m := map[string]controller.Controller{}
			if all, err := enrollment.NewTypedControllers(scope, options)(); err == nil {
				for k, p := range all {
					m[k] = controller.Singleton(p, leader)
				}
			}
			return m, nil
		},
	}

	return
}
