package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/resource"
	instance_client "github.com/docker/infrakit/pkg/rpc/instance"
	resource_server "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Resource server",
	}
	name := cmd.Flags().String("name", "resource", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

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

		resourcePlugin := resource.NewResourcePlugin(instancePluginLookup)

		cli.RunPlugin(*name, resource_server.PluginServer(resourcePlugin))

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
