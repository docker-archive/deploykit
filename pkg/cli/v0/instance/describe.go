package instance

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Describe returns the describe command
func Describe(name string, services *cli.Services) *cobra.Command {
	describe := &cobra.Command{
		Use:   "describe",
		Short: "Describe all managed instances across all groups, subject to filter",
	}
	describe.Flags().AddFlagSet(services.OutputFlags)

	tags := describe.Flags().StringSlice("tags", []string{}, "Tags to filter")
	properties := describe.Flags().BoolP("properties", "p", false, "Also returns current status/ properties")
	tagsTemplate := describe.Flags().StringP("tags-view", "t", "*", "Template to render tags")
	propertiesTemplate := describe.Flags().StringP("properties-view", "v", "{{.}}", "Template to render properties")

	quiet := describe.Flags().BoolP("quiet", "q", false, "Print rows without column headers")

	describe.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}

		instancePlugin, err := LoadPlugin(services.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(instancePlugin, "instance plugin not found", "name", name)

		options := template.Options{}
		tagsView, err := template.NewTemplate(template.ValidURL(*tagsTemplate), options)
		if err != nil {
			return err
		}
		propertiesView, err := template.NewTemplate(template.ValidURL(*propertiesTemplate), options)
		if err != nil {
			return err
		}

		filter := map[string]string{}
		for _, t := range *tags {
			p := strings.Split(t, "=")
			if len(p) == 2 {
				filter[p[0]] = p[1]
			} else {
				filter[p[0]] = ""
			}
		}

		desc, err := instancePlugin.DescribeInstances(filter, *properties)
		if err != nil {
			return err
		}

		return services.Output(os.Stdout, desc,
			func(w io.Writer, v interface{}) error {
				if !*quiet {
					if *properties {
						fmt.Printf("%-30s\t%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS", "PROPERTIES")

					} else {
						fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
					}
				}
				for _, d := range desc {

					logical := "  -   "
					if d.LogicalID != nil {
						logical = string(*d.LogicalID)
					}

					tagViewBuff := ""
					if *tagsTemplate == "*" {
						// default -- this is a hack
						printTags := []string{}
						for k, v := range d.Tags {
							printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
						}
						sort.Strings(printTags)
						tagViewBuff = strings.Join(printTags, ",")
					} else {
						tagViewBuff = renderTags(d.Tags, tagsView)
					}

					if *properties {
						fmt.Printf("%-30s\t%-30s\t%-30s\t%-s\n", d.ID, logical, tagViewBuff,
							renderProperties(d.Properties, propertiesView))
					} else {
						fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, tagViewBuff)
					}
				}
				return nil
			})
	}
	return describe
}

func renderTags(m map[string]string, view *template.Template) string {
	buff, err := view.Render(m)
	if err != nil {
		return err.Error()
	}
	return buff
}

func renderProperties(properties *types.Any, view *template.Template) string {
	var v interface{}
	err := properties.Decode(&v)
	if err != nil {
		return err.Error()
	}

	buff, err := view.Render(v)
	if err != nil {
		return err.Error()
	}
	return buff
}
