package manager

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Enforce returns the enforce command
func Enforce(name string, services *cli.Services) *cobra.Command {
	enforce := &cobra.Command{
		Use:   "enforce <global specs url>",
		Short: "Enforce global stack specification. Read from stdin if url is '-'",
	}
	enforce.Flags().AddFlagSet(services.ProcessTemplateFlags)

	enforce.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		fmt.Println("coming soon")

		return nil
	}
	return enforce
}
