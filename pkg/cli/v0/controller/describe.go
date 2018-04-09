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
	view := describe.Flags().StringP("view", "v",
		"{{ default .instance.ID .error }}", "View template for collection object states")

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

		collections, err := controller.Describe(q)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, collections,
			func(w io.Writer, v interface{}) error {

				if !*objectsOnly {
					fmt.Printf("%-10s  %-15s  %-15s\n", "KIND", "NAME", "ID")
					for _, o := range collections {

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

				// show collections only from spec.State
				format := "%-20s  %-20s  %-20s  %-20s\n"
				rows := map[string]string{}
				keys := types.Paths{}

				// collect all the fsm from each collection
				for i, c := range collections {

					if c.State == nil {
						key := types.PathFrom(c.Spec.Metadata.Name, fmt.Sprintf("%v", i))
						keys = append(keys, key)
						rows[key.String()] = fmt.Sprintf(format, c.Spec.Metadata.Name, "-", "-", "-")
						continue
					}

					// structured form -- controller/internal/Item
					type fsm struct {
						Key   string
						State string
						Data  map[string]interface{}
					}

					list := []fsm{}
					err := c.State.Decode(&list)

					if err != nil {
						// print error instead of data
						key := types.PathFrom(c.Spec.Metadata.Name, fmt.Sprintf("%v", i))
						keys = append(keys, key)
						rows[key.String()] = fmt.Sprintf(format, c.Spec.Metadata.Name, "-", "-", err.Error())
						continue

					}

					for _, l := range list {

						data := "-"
						data, err = render.Render(l.Data)
						if err != nil {
							data = err.Error()
						}

						key := types.PathFrom(c.Spec.Metadata.Name, l.Key)
						keys = append(keys, key)
						rows[key.String()] = fmt.Sprintf(format, c.Spec.Metadata.Name, l.Key, l.State, data)
					}
				}

				keys.Sort()
				fmt.Printf(format, "COLLECTION", "KEY", "STATE", "DATA")
				for _, k := range keys {
					fmt.Print(rows[k.String()])
				}

				return nil

			})
	}
	return describe
}
