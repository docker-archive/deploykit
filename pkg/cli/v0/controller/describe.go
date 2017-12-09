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

// Describe returns the describe command
func Describe(name string, services *cli.Services) *cobra.Command {
	describe := &cobra.Command{
		Use:   "describe",
		Short: "Describe all managed objects",
	}
	describe.Flags().AddFlagSet(services.OutputFlags)

	tags := describe.Flags().StringSlice("tags", []string{}, "Tags to filter")

	describe.RunE = func(cmd *cobra.Command, args []string) error {

		pluginName := plugin.Name(name)

		controller, err := services.Scope.Controller(pluginName.String())
		if err != nil {
			return nil
		}
		cli.MustNotNil(controller, "controller not found", "name", name)

		var q *types.Metadata

		if len(args) == 1 {
			s := (types.Metadata{
				Name: args[0],
			}).AddTagsFromStringSlice(*tags)
			q = &s
		}

		objects, err := controller.Describe(q)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, objects,
			func(w io.Writer, v interface{}) error {
				fmt.Printf("%-10s  %-15s  %-15s\n", "KIND", "NAME", "ID")
				for _, o := range objects {
					fmt.Printf("%-10s  %-15s  %-15s\n", o.Spec.Kind, o.Spec.Metadata.Name, o.Spec.Metadata.Identity.ID)
				}
				return nil
			})
	}
	return describe
}
