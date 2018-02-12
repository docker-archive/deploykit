package gc

import (
	"github.com/docker/infrakit/pkg/controller/gc"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "gc"
)

var (
	// EnvGCInterval sets the gc interval
	EnvGCInterval = "INFRAKIT_GC_SYNC_INTERVAL"

	log = logutil.New("module", "run/v0/gc")

	defaultOptions = gc.DefaultOptions
)

func init() {

	// We let the user set some environment variables to override
	// the default values.  These default options are then overridden
	// after the plugin started if the user provides options in the spec
	// to override them.
	if d := types.MustParseDuration(local.Getenv(EnvGCInterval, "0s")); d > 0 {
		defaultOptions.GCInterval = d
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

	transport.Name = name

	gc := gc.NewController(scope,
		func() stack.Leadership {
			return leadership(scope.Plugins)
		}, options)

	impls = map[run.PluginCode]interface{}{
		run.Controller: gc,
	}

	return
}
