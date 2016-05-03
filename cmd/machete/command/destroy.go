package command

import (
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

func destroyCmd(output console.Console, registry *provisioners.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "destroy machine_id create provisioner/template",
		Short: "destroy a machine",
		Long:  "permanently terminates a machine and removes associated local state",
		RunE: func(cmd *cobra.Command, args []string) error {
			output.Println("I can't do that yet!")
			return nil
		},
	}
}
