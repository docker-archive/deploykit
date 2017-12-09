package manager

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Inspect returns the inspect command
func Inspect(name string, services *cli.Services) *cobra.Command {
	inspect := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect returns the specs for the entire stack",
	}

	inspect.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		fmt.Println("coming soon")
		return nil
	}
	return inspect
}
