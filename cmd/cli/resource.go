package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	resource_plugin "github.com/docker/infrakit/pkg/plugin/resource"
	instance_client "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func resourceCommand(plugins func() discovery.Plugins) *cobra.Command {

	var resourcePlugin resource.Plugin

	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Access resource plugin",
	}
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {

		plugins, err := discovery.NewPluginDiscovery()
		if err != nil {
			return err
		}

		instancePluginLookup := func(n plugin.Name) (instance.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return instance_client.NewClient(n, endpoint.Address)
		}

		resourcePlugin = resource_plugin.NewResourcePlugin(instancePluginLookup)
		return nil
	}

	commit := cobra.Command{
		Use:   "commit <template URL>",
		Short: "commit a resource configuration specified by the URL",
	}
	commitPretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commit.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		templateURL := args[0]

		log.Infof("Reading template from %v", templateURL)
		engine, err := template.NewTemplate(templateURL, template.Options{
			SocketDir: discovery.Dir(),
		})
		if err != nil {
			return err
		}
		view, err := engine.AddFunc("resource", resourceIdentityFunc).Render(nil)
		if err != nil {
			return err
		}

		log.Debugln(view)

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			log.Warningln("Error parsing template")
			return err
		}

		details, err := resourcePlugin.Commit(spec, *commitPretend)
		if err == nil {
			if *commitPretend {
				fmt.Printf("Committing %s would involve: %s\n", spec.ID, details)
			} else {
				fmt.Printf("Committed %s: %s\n", spec.ID, details)
			}
		}
		return err
	}
	cmd.AddCommand(&commit)

	destroy := cobra.Command{
		Use:   "destroy <template URL>",
		Short: "destroy a resource configuration specified by the URL",
	}
	destroyPretend := destroy.Flags().Bool("pretend", false, "Don't actually destroy, only explain the destroy")
	destroy.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		templateURL := args[0]

		log.Infof("Reading template from %v", templateURL)
		engine, err := template.NewTemplate(templateURL, template.Options{
			SocketDir: discovery.Dir(),
		})
		if err != nil {
			return err
		}
		view, err := engine.AddFunc("resource", resourceIdentityFunc).Render(nil)
		if err != nil {
			return err
		}

		log.Debugln(view)

		spec := resource.Spec{}
		if err := types.AnyString(view).Decode(&spec); err != nil {
			log.Warningln("Error parsing template")
			return err
		}

		details, err := resourcePlugin.Destroy(spec, *destroyPretend)
		if err == nil {
			if *destroyPretend {
				fmt.Printf("Destroying %s would involve: %s\n", spec.ID, details)
			} else {
				fmt.Printf("Destroyed %s: %s\n", spec.ID, details)
			}
		}
		return err
	}
	cmd.AddCommand(&destroy)

	return cmd
}

func resourceIdentityFunc(name string) string {
	return fmt.Sprintf("{{ resource `%s` }}", name)
}
