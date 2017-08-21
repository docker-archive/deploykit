package group

import (
	"fmt"
	"os"
	"strconv"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/spf13/cobra"
)

// Size returns the size command
func Size(name string, services *cli.Services) *cobra.Command {

	size := &cobra.Command{
		Use:   "size <groupID>",
		Short: "Returns size of a group",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) < 1 {
				cmd.Usage()
				os.Exit(1)
			}

			groupPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(args[0])

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

			if len(args) < 2 {
				cmd.Usage()
				os.Exit(1)
			}

			groupPlugin, err := LoadPlugin(services.Plugins(), name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

			groupID := group.ID(args[0])
			target, err := strconv.Atoi(args[1])
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
