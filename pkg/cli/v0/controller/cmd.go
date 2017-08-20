package controller

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	controller_rpc "github.com/docker/infrakit/pkg/rpc/controller"
)

var log = logutil.New("module", "cli/v0/controller")

func init() {
	cli.Register(controller.InterfaceSpec,
		[]cli.CmdBuilder{
			Describe,
		})
}

// Load loads the typed object
func Load(plugins discovery.Plugins, name string) (controller.Controller, error) {
	pn := plugin.Name(name)
	endpoint, err := plugins.Find(pn)
	if err != nil {
		return nil, err
	}
	return controller_rpc.NewClient(pn, endpoint.Address)
}
