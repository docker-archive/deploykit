package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/infrakit/discovery"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/docker/infrakit/spi/instance"
	"github.com/spf13/cobra"
)

func instancePluginCommand(pluginDir func() *discovery.Dir) *cobra.Command {

	name := ""
	var instancePlugin instance.Plugin

	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Access instance plugin",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {

			callable, err := pluginDir().PluginByName(name)
			if err != nil {
				return err
			}
			instancePlugin = instance_plugin.PluginClient(callable)

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&name, "name", name, "Name of plugin")

	validate := &cobra.Command{
		Use:   "validate",
		Short: "validate input",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", instancePlugin)

			err := instancePlugin.Validate(json.RawMessage(getInput(args)))
			if err == nil {
				fmt.Println("validate:ok")
			}
			return err
		},
	}

	provision := &cobra.Command{
		Use:   "provision",
		Short: "provision the resource instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", instancePlugin)

			buff := getInput(args)
			spec := instance.Spec{}
			err := json.Unmarshal(buff, &spec)
			if err != nil {
				return err
			}

			id, err := instancePlugin.Provision(spec)
			if err == nil && id != nil {
				fmt.Printf("%s\n", *id)
			}
			return err
		},
	}

	destroy := &cobra.Command{
		Use:   "destroy",
		Short: "destroy the resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", instancePlugin)

			if len(args) == 0 {
				return errors.New("missing id")
			}

			instanceID := instance.ID(args[0])
			err := instancePlugin.Destroy(instanceID)

			if err == nil {
				fmt.Println("destroyed", instanceID)
			}
			return err
		},
	}

	tags := []string{}
	describe := &cobra.Command{
		Use:   "describe",
		Short: "describe the instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", instancePlugin)

			filter := map[string]string{}
			for _, t := range tags {
				p := strings.Split(t, "=")
				if len(p) == 2 {
					filter[p[0]] = p[1]
				} else {
					filter[p[0]] = ""
				}
			}

			desc, err := instancePlugin.DescribeInstances(filter)
			if err == nil {

				fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
				for _, d := range desc {
					logical := "  -   "
					if d.LogicalID != nil {
						logical = string(*d.LogicalID)
					}

					printTags := []string{}
					for k, v := range d.Tags {
						printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
					}
					sort.Strings(printTags)

					fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, strings.Join(printTags, ","))
				}
			}

			return err
		},
	}
	describe.Flags().StringSliceVar(&tags, "tags", tags, "Tags to filter")

	cmd.AddCommand(validate, provision, destroy, describe)

	return cmd
}
