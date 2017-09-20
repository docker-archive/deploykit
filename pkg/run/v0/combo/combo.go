package combo

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/combo"
	flavor_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// Kind is the canonical name of the plugin for starting up, etc.
	Kind = "combo"
)

var log = logutil.New("module", "run/v0/combo")

func init() {
	inproc.Register(Kind, Run, combo.DefaultOptions)
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := combo.DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name

	flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
		endpoint, err := plugins().Find(n)
		if err != nil {
			return nil, err
		}
		return flavor_rpc.NewClient(n, endpoint.Address)
	}

	impls = map[run.PluginCode]interface{}{
		run.Flavor: combo.NewPlugin(flavorPluginLookup, options),
	}
	return
}
