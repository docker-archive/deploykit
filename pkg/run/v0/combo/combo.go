package combo

import (
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/combo"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/scope"
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
func Run(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := combo.DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name

	flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
		return scope.Flavor(n.String())
	}

	impls = map[run.PluginCode]interface{}{
		run.Flavor: combo.NewPlugin(flavorPluginLookup, options),
	}
	return
}
