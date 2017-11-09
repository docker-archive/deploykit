package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Destroy returns the Destroy command
func Destroy(name string, services *cli.Services) *cobra.Command {

	destroy := &cobra.Command{
		Use:   "destroy <group ID>",
		Short: "Destroy a group by terminating and removing all members from infrastructure",
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
			err = groupPlugin.DestroyGroup(groupID)
			if err != nil {
				return err
			}

			fmt.Println("Destroy", groupID, "initiated")
			return nil
		},
	}
	return destroy
}
