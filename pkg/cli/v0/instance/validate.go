package instance

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Validate returns the validate command
func Validate(name string, services *cli.Services) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "validate <flavor configuration url>",
		Short: "Validates an flavor config.  Read from stdin if url is '-'",
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

			err = instancePlugin.Validate(types.AnyString(view))
			if err == nil {
				fmt.Println("validate:ok")
			}
			return err
		},
	}
	cmd.Flags().AddFlagSet(services.ProcessTemplateFlags)
	return cmd
}
