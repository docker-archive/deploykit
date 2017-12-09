package group

import (
	"fmt"
	"os"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Scale returns the scale command
func Scale(name string, services *cli.Services) *cobra.Command {

	scale := &cobra.Command{
		Use:   "scale <groupID> [new-target]",
		Short: "Returns size of the group if no args provided. Otherwise set the target size.",
		RunE: func(cmd *cobra.Command, args []string) error {

			pluginName := plugin.Name(name)
			_, gid := pluginName.GetLookupAndType()

			size := -1

			if gid == "" {
				// if gid is not known, then we need it to be provided
				switch len(args) {

				case 0:
					cmd.Usage()
					os.Exit(1)
				case 1:
					gid = args[0]
				case 2:
					gid = args[0]
					sz, err := strconv.Atoi(args[1])
					if err != nil {
						return err
					}
					size = sz
				default:
					cmd.Usage()
					os.Exit(1)
				}
			} else {
				// if gid is not known, then we need it to be provided
				switch len(args) {

				case 0:
					size = -1
				case 1:
					sz, err := strconv.Atoi(args[0])
					if err != nil {
						return err
					}
					size = sz
				default:
					cmd.Usage()
					os.Exit(1)
				}
			}

			groupPlugin, err := services.Scope.Group(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)
			target, err := groupPlugin.Size(groupID)
			if err != nil {
				return err
			}
			fmt.Printf("Group %v at %d instances", groupID, target)

			if size > -1 {
				err = groupPlugin.SetSize(groupID, size)
				if err != nil {
					return err
				}
				fmt.Printf(", scale to %d", size)
			}

			fmt.Println()
			return nil
		},
	}
	return scale
}
