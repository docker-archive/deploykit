package flavor

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
)

var log = logutil.New("module", "cli/v1/flavor")

func init() {
	cli.Register(flavor.InterfaceSpec,
		[]cli.CmdBuilder{
			Validate,
			Prepare,
			Healthy,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (flavor.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return flavor_plugin.NewClient(plugin.Name(name), endpoint.Address)
}
