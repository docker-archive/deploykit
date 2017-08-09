package instance

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
)

var log = logutil.New("module", "cli/v1/instance")

func init() {
	cli.Register(instance.InterfaceSpec,
		[]cli.CmdBuilder{
			Validate,
			Provision,
			Describe,
			Destroy,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (instance.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return instance_plugin.NewClient(plugin.Name(name), endpoint.Address)
}
