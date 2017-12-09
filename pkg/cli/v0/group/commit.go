package group

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Commit returns the Commit command
func Commit(name string, services *cli.Services) *cobra.Command {

	commit := &cobra.Command{
		Use:   "commit <group configuration url>",
		Short: "Commit a group configuration in LEGACY schema. Read from stdin if url is '-'",
	}

	pretend := false
	commit.Flags().BoolVar(&pretend, "pretend", pretend, "Don't actually commit, only explain where appropriate")

	commit.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		groupPlugin, err := services.Scope.Group(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(groupPlugin, "group plugin not found", "name", name)

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		spec := group.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		details, err := groupPlugin.CommitGroup(spec, pretend)
		if err != nil {
			return err
		}

		if pretend {
			fmt.Printf("Committing %s would involve: %s\n", spec.ID, details)
		} else {
			fmt.Printf("Committed %s: %s\n", spec.ID, details)
		}
		return nil
	}
	commit.Flags().AddFlagSet(services.ProcessTemplateFlags)
	return commit
}
