package resource

import (
	"github.com/docker/infrakit/pkg/controller/resource"
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
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "resource"
)

var (
	log = logutil.New("module", "run/v0/resource")

	defaultOptions = resource.DefaultOptions
)

func init() {

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

	resource := resource.NewController(scope,
		func() stack.Leadership {
			return leadership(scope.Plugins)
		}, options)

	impls = map[run.PluginCode]interface{}{
		run.Controller: resource.Controllers,
		run.Metadata:   resource.Metadata,
	}

	return
}
