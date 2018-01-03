package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Free returns the Free command
func Free(name string, services *cli.Services) *cobra.Command {

	free := &cobra.Command{
		Use:   "free-group <group ID>",
		Short: "Free a group nonedestructively from active monitoring",
		RunE: func(cmd *cobra.Command, args []string) error {

			pluginName := plugin.Name(name)
			_, gid := pluginName.GetLookupAndType()
			if gid == "" {
				if len(args) < 1 {
					cmd.Usage()
					os.Exit(1)
				} else {
					gid = args[0]
				}
			}

			groupPlugin, err := services.Scope.Group(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)
			err = groupPlugin.FreeGroup(groupID)
			if err != nil {
				return err
			}

			fmt.Println("Freed", groupID)
			return nil
		},
	}
	free.Flags().AddFlagSet(services.OutputFlags)
	return free
}
