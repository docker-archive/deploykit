package vanilla

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/launch/inproc"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/flavor/vanilla"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// CanonicalName is the canonical name of the plugin and also key used to locate the plugin in discovery
	CanonicalName = "flavor-vanilla"
)

var (
	log = logutil.New("module", "run/flavor/vanilla")
)

func init() {
	inproc.Register(CanonicalName, Run, DefaultOptions)
}

// Options capture the options for starting up the group controller.
type Options struct {
	template.Options `json:",inline" yaml:",inline"`

	// Name of the plugin
	Name string
}

// DefaultOptions return an Options with default values filled in.
var DefaultOptions = Options{
	Name: CanonicalName,
	Options: template.Options{
		MultiPass: true,
	},
}

// Run runs the plugin, blocking the current thread.  Error is returned immediately
// if the plugin cannot be started.
func Run(plugins func() discovery.Plugins,
	config *types.Any) (name plugin.Name, impls []interface{}, onStop func(), err error) {

	options := DefaultOptions
	err = config.Decode(&options)
	if err != nil {
		return
	}

	name = plugin.Name(options.Name)
	impls = []interface{}{
		vanilla.NewPlugin(options.Options),
	}
	return
}
