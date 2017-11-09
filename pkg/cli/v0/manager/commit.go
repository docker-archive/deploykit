package manager

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Commit returns the commit command
func Commit(name string, services *cli.Services) *cobra.Command {
	commit := &cobra.Command{
		Use:   "commit <global specs url>",
		Short: "Commit global stack specification. Read from stdin if url is '-'",
	}
	commit.Flags().AddFlagSet(services.ProcessTemplateFlags)

	commit.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		return nil
	}
	return commit
}
