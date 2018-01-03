package controller

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/v0/controller")

func init() {
	cli.Register(controller.InterfaceSpec,
		[]cli.CmdBuilder{
			//Controller,
			Describe,
			Commit,
			Free,
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
		Free(name, services),
	)

	return controller
}
