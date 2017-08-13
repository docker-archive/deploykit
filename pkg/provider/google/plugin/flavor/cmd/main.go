package main

import (
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/google/plugin/flavor"
	flavor_client "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/run"
	flavor_plugin "github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "A Flavor plugin that supports composition of other Flavors",
	}

	name := cmd.Flags().String("name", "flavor-combo", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	project := cmd.Flags().String("project", "", "Google Cloud project")
	zone := cmd.Flags().String("zone", "", "Google Cloud zone")
	minAge := cmd.Flags().Duration("minAge", 0, "Min age to be considered healthy")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)

		plugins, err := local.NewPluginDiscovery()
		if err != nil {
			return err
		}

		flavorPluginLookup := func(n plugin.Name) (flavor_plugin.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return flavor_client.NewClient(n, endpoint.Address)
		}

		run.Plugin(plugin.DefaultTransport(*name),
			flavor_client.PluginServer(flavor.NewPlugin(flavorPluginLookup, *project, *zone, *minAge)))

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
