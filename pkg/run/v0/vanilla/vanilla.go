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

const (
	// CanonicalName is the canonical name of the plugin for starting up, etc.
	CanonicalName = "vanilla"
)

var log = logutil.New("module", "run/v0/vanilla")

func init() {
	inproc.Register("vanilla", Run, DefaultOptions)
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
	Name: CanonicalName,
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (transport plugin.Transport, impls map[run.PluginCode]interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	transport.Name = plugin.Name(options.Name)
	impls = map[run.PluginCode]interface{}{
		run.Flavor: vanilla.NewPlugin(options.Options),
	}
	return
}