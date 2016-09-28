package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/libmachete/discovery"
	"github.com/docker/libmachete/spi/flavor"
	flavor_plugin "github.com/docker/libmachete/spi/http/flavor"
	"github.com/docker/libmachete/spi/instance"
	"github.com/spf13/cobra"
)

func flavorPluginCommand(pluginDir func() *discovery.Dir) *cobra.Command {

	name := ""
	var flavorPlugin flavor.Plugin

	cmd := &cobra.Command{
		Use:   "flavor",
		Short: "Access flavor plugin",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {

			callable, err := pluginDir().PluginByName(name)
			if err != nil {
				return err
			}
			flavorPlugin = flavor_plugin.PluginClient(callable)

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&name, "name", name, "Name of plugin")

	validate := &cobra.Command{
		Use:   "validate",
		Short: "validate input",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", flavorPlugin)

			alloc, err := flavorPlugin.Validate(json.RawMessage(getInput(args)))
			if err == nil {
				if buff, err2 := json.MarshalIndent(alloc, "  ", "  "); err2 == nil {
					fmt.Println("validate", string(buff))
				} else {
					err = err2
				}
			}
			return err
		},
	}

	flavorPropertiesFile := ""
	prepare := &cobra.Command{
		Use:   "prepare",
		Short: "prepare the provision data",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", flavorPlugin)

			buff := getInput(args)
			spec := instance.Spec{}
			err := json.Unmarshal(buff, &spec)
			if err != nil {
				return err
			}

			// the flavor's config is in a flag.
			buffFlavor := getInput([]string{flavorPropertiesFile})

			spec, err = flavorPlugin.Prepare(json.RawMessage(buffFlavor), spec)
			if err == nil {
				buff, err = json.MarshalIndent(spec, "  ", "  ")
				if err == nil {
					fmt.Println(string(buff))
				}
			}
			return err
		},
	}
	prepare.Flags().StringVar(&flavorPropertiesFile, "properties", flavorPropertiesFile, "Path to flavor properties")

	tags := []string{}
	id := ""
	logicalID := ""
	healthy := &cobra.Command{
		Use:   "healthy",
		Short: "checks for health",
		RunE: func(cmd *cobra.Command, args []string) error {
			assertNotNil("no plugin", flavorPlugin)

			filter := map[string]string{}
			for _, t := range tags {
				p := strings.Split(t, "=")
				if len(p) == 2 {
					filter[p[0]] = p[1]
				} else {
					filter[p[0]] = ""
				}
			}

			desc := instance.Description{}
			if len(filter) > 0 {
				desc.Tags = filter
			}
			if id != "" {
				desc.ID = instance.ID(id)
			}
			if logicalID != "" {
				logical := instance.LogicalID(logicalID)
				desc.LogicalID = &logical
			}

			healthy, err := flavorPlugin.Healthy(desc)
			if err == nil {
				fmt.Printf("%v\n", healthy)
			}
			return err
		},
	}
	healthy.Flags().StringSliceVar(&tags, "tags", tags, "Tags to filter")
	healthy.Flags().StringVar(&id, "id", id, "ID of resource")
	healthy.Flags().StringVar(&logicalID, "logical-id", logicalID, "Logical ID of resource")

	cmd.AddCommand(validate, prepare, healthy)

	return cmd
}
