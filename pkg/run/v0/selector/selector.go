package selector

import (
	"strconv"
	"strings"

	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/spread"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/tiered"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/weighted"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// KindWeighted is the canonical name of the plugin for starting up, etc.
	KindWeighted = "selector/weighted"

	// KindSpread is the canonical name of the plugin for starting up, etc.
	KindSpread = "selector/spread"

	// KindTiered is the canonical name of the plugin for starting up, etc.
	KindTiered = "selector/tiered"

	// EnvSpreadPlugins is the env to set to specifiy the plugins and labels
	// ex) aws/compute=a:b,x:y;gcp/compute=a:1,b:2
	EnvSpreadPlugins = "INFRAKIT_SELECTOR_SPREAD_PLUGINS"

	// EnvWeightedPlugins is the env to set to specifiy the weighting of plugins
	// ex) aws/compute=80;gcp/compute=20
	EnvWeightedPlugins = "INFRAKIT_SELECTOR_WEIGHTED_PLUGINS"

	// EnvTieredPlugins is the env to set to specifiy the ordered list of plugins
	// ex) spot/compute;ondemand/compute
	EnvTieredPlugins = "INFRAKIT_SELECTOR_TIERED_PLUGINS"
)

var (
	log = logutil.New("module", "run/v0/selector")
)

func init() {
	inproc.Register(KindWeighted, RunWeighted, weightedOptions())
	inproc.Register(KindSpread, RunSpread, spreadOptions())
	inproc.Register(KindTiered, RunTiered, tieredOptions())
}

func spreadOptions() selector.Options {

	// example: start up simulator at simulator:aws simulator:gcp
	list := local.Getenv(EnvSpreadPlugins, "aws/compute=a:b,x:y;gcp/compute=a:1,b:2")

	options := selector.Options{}
	for _, s := range strings.Split(list, ";") {

		p := strings.Split(s, "=")
		n := plugin.Name(p[0])

		choice := selector.Choice{Name: n}
		if len(p) > 1 {
			// build the map from the string of the form a:b,...
			t := map[string]string{}
			for _, kv := range strings.Split(p[1], ",") {
				kvp := strings.Split(kv, ":")
				t[kvp[0]] = kvp[1]
			}
			choice.Affinity = types.AnyValueMust(spread.AffinityArgs{Labels: t})
		}
		options = append(options, choice)
	}
	return options
}

func weightedOptions() selector.Options {

	// example: start up simulator at simulator:aws simulator:gcp
	list := local.Getenv(EnvWeightedPlugins, "aws/compute=80;gcp/compute=20")

	options := selector.Options{}
	for _, s := range strings.Split(list, ";") {

		p := strings.Split(s, "=")
		n := plugin.Name(p[0])
		w, err := strconv.Atoi(p[1])
		if err != nil {
			panic(err)
		}

		options = append(options, selector.Choice{
			Name:     n,
			Affinity: types.AnyValueMust(weighted.AffinityArgs{Weight: uint(w)}),
		})
	}
	return options
}

func tieredOptions() selector.Options {

	// Start up simulator at two endpoints:  start simulator:spot simulator:ondemand
	list := local.Getenv(EnvTieredPlugins, "") //"spot/compute;ondemand/compute")

	options := selector.Options{}
	for _, n := range strings.Split(list, ";") {
		options = append(options, selector.Choice{
			Name: plugin.Name(n),
		})
	}
	return options
}

// RunWeighted runs the plugin, blocking the current thread.  Error is returned immediately if start fails
func RunWeighted(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := selector.Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"weighted": weighted.NewPlugin(scope.Plugins, options),
		},
	}
	return
}

// RunSpread runs the plugin, blocking the current thread.  Error is returned immediately if start fails
func RunSpread(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := selector.Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"spread": spread.NewPlugin(scope.Plugins, options),
		},
	}
	return
}

// RunTiered runs the plugin, blocking the current thread.  Error is returned immediately if start fails
func RunTiered(scope scope.Scope, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := selector.Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: map[string]instance.Plugin{
			"tiered": tiered.NewPlugin(scope.Plugins, options),
		},
	}
	return
}
