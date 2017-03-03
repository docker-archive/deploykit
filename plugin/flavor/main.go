package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	flavor_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "A Flavor plugin that supports composition of other Flavors",
	}
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	name := cmd.Flags().String("name", "flavor-combo", "Plugin name to advertise for discovery")
	cmd.Run = func(c *cobra.Command, args []string) {

		plugins, err := discovery.NewPluginDiscovery()
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return flavor_rpc.NewClient(n, endpoint.Address)
		}

		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, flavor_rpc.PluginServer(NewPlugin(flavorPluginLookup)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
