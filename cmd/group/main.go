package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group"
	flavor_client "github.com/docker/infrakit/pkg/rpc/flavor"
	group_server "github.com/docker/infrakit/pkg/rpc/group"
	instance_client "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Group server",
	}
	name := cmd.Flags().String("name", "group", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	pollInterval := cmd.Flags().Duration("poll-interval", 10*time.Second, "Group polling interval")
	maxParallelNum := cmd.Flags().Uint("max-parallel", 0, "Max number of parallel instance operation (create, delete). (Default: 0 = no limit)")
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
			return instance_client.NewClient(plugin.Name(n), endpoint.Address), nil
		}

		flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return flavor_client.NewClient(endpoint.Address), nil
		}

		cli.RunPlugin(*name, group_server.PluginServer(
			group.NewGroupPlugin(instancePluginLookup, flavorPluginLookup, *pollInterval, *maxParallelNum)))

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
