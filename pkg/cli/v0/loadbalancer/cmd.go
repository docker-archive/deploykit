package loadbalancer

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	loadbalancer_rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var log = logutil.New("module", "cli/v0/loadbalancer")

func init() {
	cli.Register(loadbalancer.InterfaceSpec,
		[]cli.CmdBuilder{
			Routes,
			Backends,
		})
}

// Load loads the typed object
func Load(plugins discovery.Plugins, name string) (loadbalancer.L4, error) {
	pn := plugin.Name(name)
	endpoint, err := plugins.Find(pn)
	if err != nil {
		return nil, err
	}
	return loadbalancer_rpc.NewClient(pn, endpoint.Address)
}
