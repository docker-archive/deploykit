package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	"github.com/docker/infrakit/discovery"
	flavor_rpc "github.com/docker/infrakit/rpc/flavor"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/spf13/cobra"
)

func main() {

	var logLevel int
	var name string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "A Flavor plugin that supports composition of other Flavors",
		Run: func(c *cobra.Command, args []string) {

			plugins, err := discovery.NewPluginDiscovery()
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			flavorPluginLookup := func(n string) (flavor.Plugin, error) {
				endpoint, err := plugins.Find(n)
				if err != nil {
					return nil, err
				}
				return flavor_rpc.NewClient(endpoint.Protocol, endpoint.Address)
			}

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, flavor_rpc.PluginServer(NewPlugin(flavorPluginLookup)))
		},
	}

	cmd.AddCommand(cli.VersionCommand())

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&name, "name", "flavor-combo", "Plugin name to advertise for discovery")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
