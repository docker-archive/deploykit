package controller

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Commit returns the commit command
func Commit(name string, services *cli.Services) *cobra.Command {
	commit := &cobra.Command{
		Use:   "commit <group configuration url>",
		Short: "Commit a group configuration. Read from stdin if url is '-'",
	}
	commit.Flags().AddFlagSet(services.OutputFlags)

	commit.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		c, err := Load(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(c, "controller not found", "name", name)

		view, err := services.ReadFromStdinIfElse(
			func() bool { return args[0] == "-" },
			func() (string, error) { return services.ProcessTemplate(args[0]) },
			services.ToJSON,
		)
		if err != nil {
			return err
		}

		spec := types.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			return err
		}

		object, err := c.Commit(controller.Manage, spec)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, object,
			func(w io.Writer, v interface{}) error {

				buff, err := types.AnyValueMust(v).MarshalYAML()
				if err != nil {
					return err
				}
				fmt.Printf("%v\n", string(buff))
				return nil
			})
	}
	return commit
}
