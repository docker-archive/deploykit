package command

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

type create struct {
	output console.Console
}

func (c *create) run(args []string) error {
	return NotImplementedError
}

func createCmd(
	output console.Console,
	registry *provisioners.Registry,
	templates libmachete.TemplateLoader) *cobra.Command {

	cmd := create{output: output}

	return &cobra.Command{
		Use:   "create provisioner template",
		Short: "create a machine",
		RunE: func(_ *cobra.Command, args []string) error {
			return cmd.run(args)
		},
	}
}
