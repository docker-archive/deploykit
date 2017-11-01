package resource

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Describe returns the Describe command
func Describe(name string, services *cli.Services) *cobra.Command {

	describe := &cobra.Command{
		Use:   "describe <template URL>",
		Short: "Describe a resource configuration specified by the URL. Read from stdin if url is '-'",
	}

	describe.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		resourcePlugin, err := LoadPlugin(services.Scope.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(resourcePlugin, "resource plugin not found", "name", name)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		details, err := resourcePlugin.DescribeResources(spec)
		if err != nil {
			return err
		}
		if len(details) == 0 {
			return nil
		}

		return services.Output(os.Stdout, details, nil)
	}
	describe.Flags().AddFlagSet(services.OutputFlags)
	return describe
}
