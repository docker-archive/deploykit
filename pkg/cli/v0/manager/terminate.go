package manager

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Terminate returns the terminate command
func Terminate(name string, services *cli.Services) *cobra.Command {
	terminate := &cobra.Command{
		Use:   "terminate",
		Short: "Terminate returns the specs for the entire stack",
	}

	terminate.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		fmt.Println("coming soon")
		return nil
	}
	return terminate
}
