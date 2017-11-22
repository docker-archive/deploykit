package manager

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/spf13/cobra"
)

// Specs returns the specs command
func Specs(name string, services *cli.Services) *cobra.Command {
	specs := &cobra.Command{
		Use:   "specs",
		Short: "Specs returns the specs for the entire stack",
	}

	specs.Flags().AddFlagSet(services.OutputFlags)
	specs.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		stack, err := services.Scope.Stack(name)
		if err != nil {
			return err
		}

		specs, err := stack.Specs()
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, specs, nil)
	}
	return specs
}
