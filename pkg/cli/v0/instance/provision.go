package instance

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Provision returns the provision command
func Provision(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "provision <instance configuration url>",
		Short: "Provisions an instance.  Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			instancePlugin, err := services.Scope.Instance(name)
			if err != nil {
				return nil
			}
			cli.MustNotNil(instancePlugin, "instance plugin not found", "name", name)

			view, err := services.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return services.ProcessTemplate(args[0]) },
				services.ToJSON,
			)
			if err != nil {
				return err
			}

			spec := instance.Spec{}
			if err := types.AnyString(view).Decode(&spec); err != nil {
				return err
			}

			id, err := instancePlugin.Provision(spec)
			if err == nil && id != nil {
				fmt.Printf("%s\n", *id)
			}
			return err
		},
	}
	cmd.Flags().AddFlagSet(services.ProcessTemplateFlags)
	return cmd
}
