package group

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Groups returns the Groups command
func Groups(name string, services *cli.Services) *cobra.Command {

	groups := &cobra.Command{
		Use:   "groups",
		Short: "List groups",
	}

	quiet := groups.Flags().BoolP("quiet", "q", false, "Print rows without column headers")

	groups.RunE = func(cmd *cobra.Command, args []string) error {

		groupPlugin, err := services.Scope.Group(name)
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
	return groups
}
