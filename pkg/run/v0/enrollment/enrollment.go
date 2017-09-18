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

func leadership(plugins func() discovery.Plugins) (manager.Leadership, error) {
	// Scan for a manager
	pm, err := plugins().List()
	if err != nil {
		return nil, err
	}

	for _, endpoint := range pm {
		rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
		if err == nil {
			return manager_rpc.Adapt(rpcClient), nil
		}
	}
	return nil, nil
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	if plugins == nil {
		panic("no plugins()")
	}

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	log.Info("Decoded input", "config", options)

	leader, err := leadership(plugins)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Controller: enrollment_controller.NewTypedControllers(plugins, leader, options),
	}

	return
}
