package controller

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	controller_rpc "github.com/docker/infrakit/pkg/rpc/controller"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/v0/controller")

func init() {
	cli.Register(controller.InterfaceSpec,
		[]cli.CmdBuilder{
			Controller,
			// Describe,
			// Commit,
		})
}

// Controller returns the controller sub command
func Controller(name string, services *cli.Services) *cobra.Command {
	controller := &cobra.Command{
		Use:   "controller",
		Short: "Commands to access the Controller SPI",
	}

	controller.AddCommand(
		Describe(name, services),
		Commit(name, services),
	)

	return controller
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
