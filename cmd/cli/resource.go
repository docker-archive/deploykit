package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	resource_plugin "github.com/docker/infrakit/pkg/plugin/resource"
	instance_client "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
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
		Use:   "commit <resource configuration>",
		Short: "commit a resource configuration",
	}
	commitPretend := commit.Flags().Bool("pretend", false, "Don't actually commit, only explain the commit")
	commit.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		buff, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		spec := resource.Spec{}
		if err := json.Unmarshal(buff, &spec); err != nil {
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
		Use:   "destroy <resource configuration>",
		Short: "destroy a resource configuration",
	}
	destroyPretend := destroy.Flags().Bool("pretend", false, "Don't actually destroy, only explain the destroy")
	destroy.RunE = func(cmd *cobra.Command, args []string) error {
		assertNotNil("no plugin", resourcePlugin)

		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		buff, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		spec := resource.Spec{}
		if err := json.Unmarshal(buff, &spec); err != nil {
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
