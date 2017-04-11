package instance

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/docker/infrakit/cmd/cli/base"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/instance")

func init() {
	base.Register(Command)
}

// Command is the entry point to this module
func Command(plugins func() discovery.Plugins) *cobra.Command {

	var instancePlugin instance.Plugin

	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Access instance plugin",
	}
	name := cmd.PersistentFlags().String("name", "", "Name of plugin")
	quiet := cmd.PersistentFlags().BoolP("quiet", "q", false, "Print rows without column headers")
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := cli.EnsurePersistentPreRunE(c); err != nil {
			return err
		}

		endpoint, err := plugins().Find(plugin.Name(*name))
		if err != nil {
			return err
		}

		p, err := instance_plugin.NewClient(plugin.Name(*name), endpoint.Address)
		if err != nil {
			return err
		}
		instancePlugin = p

		cli.MustNotNil(instancePlugin, "instance plugin not found", "name", *name)
		return nil
	}

	templateFlags, toJSON, _, processTemplate := base.TemplateProcessor(plugins)

	///////////////////////////////////////////////////////////////////////////////////
	// validate
	validate := &cobra.Command{
		Use:   "validate <instance configuration url>",
		Short: "Validates an instance configuration. Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
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
	validate.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// provision
	provision := &cobra.Command{
		Use:   "provision <instance configuration url>",
		Short: "Provisions an instance.  Read from stdin if url is '-'",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			view, err := base.ReadFromStdinIfElse(
				func() bool { return args[0] == "-" },
				func() (string, error) { return processTemplate(args[0]) },
				toJSON,
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
	provision.Flags().AddFlagSet(templateFlags)

	///////////////////////////////////////////////////////////////////////////////////
	// destroy
	destroy := &cobra.Command{
		Use:   "destroy <instance ID>",
		Short: "Destroy the instance",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			instanceID := instance.ID(args[0])
			err := instancePlugin.Destroy(instanceID)

			if err == nil {
				fmt.Println("destroyed", instanceID)
			}
			return err
		},
	}

	///////////////////////////////////////////////////////////////////////////////////
	// describe
	describe := &cobra.Command{
		Use:   "describe",
		Short: "Describe all managed instances across all groups, subject to filter",
	}
	tags := describe.Flags().StringSlice("tags", []string{}, "Tags to filter")
	properties := describe.Flags().BoolP("properties", "p", false, "Also returns current status/ properties")
	tagsTemplate := describe.Flags().StringP("tags-view", "t", "*", "Template to render tags")
	propertiesTemplate := describe.Flags().StringP("properties-view", "v", "{{.}}", "Template to render properties")

	rawOutputFlags, rawOutput := base.RawOutput()
	describe.Flags().AddFlagSet(rawOutputFlags)

	describe.RunE = func(cmd *cobra.Command, args []string) error {

		tagsView, err := template.New("describe-instances-tags").Parse(*tagsTemplate)
		if err != nil {
			return err
		}
		propertiesView, err := template.New("describe-instances-properties").Parse(*propertiesTemplate)
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
		if err == nil {

			rendered, err := rawOutput(os.Stdout, desc)
			if err != nil {
				return err
			}

			if rendered {
				return nil
			}

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
		}

		return err
	}

	cmd.AddCommand(
		validate,
		provision,
		destroy,
		describe,
	)

	return cmd
}

func renderTags(m map[string]string, view *template.Template) string {
	buff := new(bytes.Buffer)
	err := view.Execute(buff, m)
	if err != nil {
		return err.Error()
	}
	return buff.String()
}

func renderProperties(properties *types.Any, view *template.Template) string {
	var v interface{}
	err := properties.Decode(&v)
	if err != nil {
		return err.Error()
	}

	buff := new(bytes.Buffer)
	err = view.Execute(buff, v)
	if err != nil {
		return err.Error()
	}
	return buff.String()
}
