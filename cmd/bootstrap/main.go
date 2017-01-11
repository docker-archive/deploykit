package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin/bootstrap"
	group_server "github.com/docker/infrakit/pkg/rpc/group"
	resource_client "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Bootstrap server",
	}
	name := cmd.Flags().String("name", "bootstrap", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

		plugins, err := discovery.NewPluginDiscovery()
		if err != nil {
			return err
		}

		resourcePluginLookup := func(n string) (resource.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return resource_client.NewClient(endpoint.Address), nil
		}

		cli.RunPlugin(*name, group_server.PluginServer(bootstrap.NewBootstrapPlugin(resourcePluginLookup)))

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
