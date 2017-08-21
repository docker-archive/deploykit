package controller

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
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
	objectName := describe.Flags().String("name", "", "Name of object")
	objectID := describe.Flags().String("id", "", "ID of object")

	describe.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		controller, err := Load(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(controller, "controller not found", "name", name)

		search := (types.Metadata{
			Name: *objectName,
		}).AddTagsFromStringSlice(*tags)

		if *objectID != "" {
			search.Identity = &types.Identity{
				ID: *objectID,
			}
		}
		objects, err := controller.Describe(&search)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, objects,
			func(w io.Writer, v interface{}) error {

				for _, o := range objects {

					buff, err := types.AnyValueMust(o).MarshalYAML()
					if err != nil {
						return err
					}
					fmt.Printf("%v\n", string(buff))
				}
				return nil
			})
	}
	return describe
}
