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

// Size returns the size command
func Size(name string, services *cli.Services) *cobra.Command {

	size := &cobra.Command{
		Use:   "size <groupID>",
		Short: "Returns size of a group",
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

			groupPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)

			target, err := groupPlugin.Size(groupID)
			if err != nil {
				return err
			}
			fmt.Println("Group", groupID, "target size=", target)
			return nil
		},
	}
	return size
}

// SetSize returns the set size command
func SetSize(name string, services *cli.Services) *cobra.Command {

	set := &cobra.Command{
		Use:   "set-size <groupID> <size>",
		Short: "Sets target size of a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			sizeArg := ""
			pluginName := plugin.Name(name)
			_, gid := pluginName.GetLookupAndType()
			if gid == "" {
				if len(args) < 2 {
					cmd.Usage()
					os.Exit(1)
				} else {
					gid = args[0]
					sizeArg = args[1]
				}
			} else {
				if len(args) < 1 {
					cmd.Usage()
					os.Exit(1)
				} else {
					sizeArg = args[0]
				}
			}

			groupPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(gid)
			target, err := strconv.Atoi(sizeArg)
			if err != nil {
				return err
			}
			err = groupPlugin.SetSize(groupID, target)
			if err != nil {
				return err
			}
			fmt.Println("Group", groupID, "set target to", target)
			return nil
		},
	}
	return set
}
