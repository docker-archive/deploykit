package controller

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Free returns the free command
func Free(name string, services *cli.Services) *cobra.Command {
	free := &cobra.Command{
		Use:   "free [name]",
		Short: "Free an object managed by the controller.",
	}
	free.Flags().AddFlagSet(services.ProcessTemplateFlags)

	free.RunE = func(cmd *cobra.Command, args []string) error {

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		c, err := services.Scope.Controller(name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(c, "controller not found", "name", name)

		search := types.Metadata{
			Name: plugin.Name(name).String(),
		}
		object, err := c.Free(&search)
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
	return free
}
