package resource

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Destroy returns the Destroy command
func Destroy(name string, services *cli.Services) *cobra.Command {

	destroy := &cobra.Command{
		Use:   "destroy <template URL>",
		Short: "Destroy a resource configuration specified by the URL. Read from stdin if url is '-'",
	}

	pretend := destroy.Flags().Bool("pretend", false, "Don't actually do changes. Explain only where appropriate")

	destroy.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		resourcePlugin, err := LoadPlugin(services.Scope.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(resourcePlugin, "resource plugin not found", "name", name)

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

		details, err := resourcePlugin.Destroy(spec, *pretend)
		if err != nil {
			return nil
		}
		if *pretend {
			fmt.Printf("Destroying %s would involve:\n%s\n", spec.ID, details)
		} else {
			fmt.Printf("Destroyed %s:\n%s\n", spec.ID, details)
		}
		return nil
	}
	return destroy
}
