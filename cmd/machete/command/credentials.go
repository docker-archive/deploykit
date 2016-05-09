package command

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

type credentials struct {
	output console.Console
}

func credentialsCmd(output console.Console,
	registry *provisioners.Registry,
	templates libmachete.Templates) *cobra.Command {

	cmd := create{
		output:         output,
		machineCreator: libmachete.NewCreator(registry, templates)}

	return &cobra.Command{
		Use:   "create provisioner template",
		Short: "create a machine",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.run(args)
		},
	}
}
