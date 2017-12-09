package manager

import (
	"fmt"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Enforce returns the enforce command
func Enforce(name string, services *cli.Services) *cobra.Command {

	pretend := true
	enforce := &cobra.Command{
		Use:   "enforce <global specs url>",
		Short: "Enforce global stack specification. Read from stdin if url is '-'",
	}
	enforce.Flags().AddFlagSet(services.ProcessTemplateFlags)
	enforce.Flags().BoolVar(&pretend, "pretend", pretend, "Don't actually commit, only explain where appropriate")

	enforce.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		stack, err := services.Scope.Stack(name)
		if err != nil {
			return err
		}
		cli.MustNotNil(stack, "stack plugin not found", "name", name)

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		specs := types.Specs{}
		if err := types.AnyString(view).Decode(&specs); err != nil {
			return err
		}

		if pretend {

			// TODO - we should implement this on the server-side, but this requires
			// a change to the stack SPI which is too large for this PR.

			before, err := stack.Specs()
			if err != nil {
				return err
			}

			changes := types.Specs(before).Changes(specs)

			for action, specs := range map[string]interface{}{
				"ADD":    changes.Add,
				"REMOVE": changes.Remove,
				"CHANGE": changes.Changes,
			} {
				fmt.Printf("\n%v\n", action)
				any := types.AnyValueMust(specs)
				buff, err := any.MarshalYAML()
				if err != nil {
					return err
				}
				fmt.Print(string(buff))
				fmt.Println()
			}

			return nil
		}

		return stack.Enforce(specs)
	}
	return enforce
}
