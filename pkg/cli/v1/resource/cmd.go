package resource

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	resource_rpc "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/resource"
)

var log = logutil.New("module", "cli/v1/resource")

func init() {
	cli.Register(resource.InterfaceSpec,
		[]cli.CmdBuilder{
			Commit,
			Describe,
			Destroy,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (resource.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return resource_rpc.NewClient(endpoint.Address)
}
