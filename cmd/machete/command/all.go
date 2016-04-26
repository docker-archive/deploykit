package command

import (
	"github.com/docker/libmachete/provisioners"
	"github.com/spf13/cobra"
)

// GetSubcommands gets all the available subcommands.
func GetSubcommands(registry *provisioners.Registry) []*cobra.Command {
	return []*cobra.Command{
		createCmd(registry),
		destroyCmd(registry),
	}
}
