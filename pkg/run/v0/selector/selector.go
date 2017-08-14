package selector

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/spread"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/weighted"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// KindWeighted is the canonical name of the plugin for starting up, etc.
	KindWeighted = "selector/weighted"

	// KindSpread is the canonical name of the plugin for starting up, etc.
	KindSpread = "selector/spread"
)

var (
	log = logutil.New("module", "run/v0/selector")
)

func init() {
	inproc.Register(KindWeighted, RunWeighted, weighted.DefaultOptions)
	inproc.Register(KindSpread, RunSpread, spread.DefaultOptions)
}

// RunWeighted runs the plugin, blocking the current thread.  Error is returned immediately if start fails
func RunWeighted(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := selector.Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: weighted.NewPlugin(plugins, options),
	}
	return
}

// RunSpread runs the plugin, blocking the current thread.  Error is returned immediately if start fails
func RunSpread(plugins func() discovery.Plugins, name plugin.Name,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := selector.Options{}
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = name
	impls = map[run.PluginCode]interface{}{
		run.Instance: spread.NewPlugin(plugins, options),
	}
	return
}
