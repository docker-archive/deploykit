package controller

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/template"
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
	objectsOnly := describe.Flags().BoolP("objects", "o", false, "True to show objects only")
	view := describe.Flags().StringP("view", "v", "{{.instance.ID}}", "View template for collection object states")

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

				if !*objectsOnly {
					fmt.Printf("%-10s  %-15s  %-15s\n", "KIND", "NAME", "ID")
					for _, o := range objects {

						id := "-"
						if o.Spec.Metadata.Identity != nil {
							id = o.Spec.Metadata.Identity.ID
						}

						fmt.Printf("%-10s  %-15s  %-15s\n", o.Spec.Kind, o.Spec.Metadata.Name, id)
					}
					return nil
				}

				render, err := services.Scope.TemplateEngine("str://"+*view, template.Options{})
				if err != nil {
					return err
				}

				// show objects only from spec.State
				fmt.Printf("%-10s  %-15s  %-15s\n", "KEY", "STATE", "DATA")
				for _, o := range objects {

					if o.State == nil {
						fmt.Printf("%-10s  %-15s  %-15s\n", o.Spec.Metadata.Name, "-", "-")
						continue
					}

					// structured form -- controller/internal/Item
					type fsm struct {
						Key   string
						State string
						Data  map[string]interface{}
					}

					list := []fsm{}
					err := o.State.Decode(&list)

					if err != nil {
						// print this like a regular object
						fmt.Printf("%-10s  %-15s  %-15v\n", o.Spec.Metadata.Name, "-", o)
						continue

					}

					for _, l := range list {

						data := "-"
						data, err = render.Render(l.Data)
						if err != nil {
							data = err.Error()
						}

						fmt.Printf("%-10s  %-15s  %-15v\n", l.Key, l.State, data)
					}
				}
				return nil

			})
	}
	return describe
}
