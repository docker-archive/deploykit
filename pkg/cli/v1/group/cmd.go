package group

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
)

var log = logutil.New("module", "cli/v1/group")

func init() {
	cli.Register(group.InterfaceSpec,
		[]cli.CmdBuilder{
			Ls,
			Inspect,
			Describe,
			Commit,
			Free,
			Destroy,
		})
}

// LoadPlugin loads the typed plugin
func LoadPlugin(plugins discovery.Plugins, name string) (group.Plugin, error) {
	endpoint, err := plugins.Find(plugin.Name(name))
	if err != nil {
		return nil, err
	}
	return group_rpc.NewClient(endpoint.Address)
}
