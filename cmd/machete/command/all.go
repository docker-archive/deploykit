package command

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/cmd/machete/console"
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

// GetSubcommands gets all the available subcommands.
func GetSubcommands(
	output console.Console,
	registry *provisioners.Registry,
	templates libmachete.TemplateLoader) []*cobra.Command {

	return []*cobra.Command{
		createCmd(output, registry, templates),
		destroyCmd(output, registry),
	}
}
