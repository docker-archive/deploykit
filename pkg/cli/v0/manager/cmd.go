package manager

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/spi/stack"
)

var log = logutil.New("module", "cli/v1/manager")

func init() {
	cli.Register(stack.InterfaceSpec,
		[]cli.CmdBuilder{
			Enforce,
			Inspect,
			Specs,
			Terminate,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (stack.Interface, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return manager_rpc.NewClient(endpoint.Address)
}
