package vanilla

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/vanilla"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "run/flavor/vanilla")

func init() {
	inproc.Register("flavor-vanilla", Run, DefaultOptions)
}

// Options capture the options for starting up the group controller.
type Options struct {
	template.Options `json:",inline" yaml:",inline"`

	// Name of the plugin
	Name string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Options: template.Options{
		MultiPass: true,
	},
	Name: "flavor-vanilla",
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (name plugin.Name, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	name = plugin.Name(options.Name)
	impls = map[run.PluginCode]interface{}{
		run.Flavor: vanilla.NewPlugin(options.Options),
	}
	return
}
