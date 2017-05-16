package group

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Ls returns the Ls command
func Ls(name string, services *cli.Services) *cobra.Command {

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List groups",
	}

	quiet := ls.Flags().BoolP("quiet", "q", false, "Print rows without column headers")

	ls.RunE = func(cmd *cobra.Command, args []string) error {

		groupPlugin, err := LoadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

		groups, err := groupPlugin.InspectGroups()
		if err == nil {
			if !*quiet {
				fmt.Printf("%s\n", "ID")
			}
			for _, g := range groups {
				fmt.Printf("%s\n", g.ID)
			}
		}

		return err
	}
	return ls
}
