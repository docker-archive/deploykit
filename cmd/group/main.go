package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	flavor_client "github.com/docker/infrakit/pkg/rpc/flavor"
	group_server "github.com/docker/infrakit/pkg/rpc/group"
	instance_client "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
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
			return instance_client.NewClient(n, endpoint.Address)
		}

		flavorPluginLookup := func(n plugin.Name) (flavor.Plugin, error) {
			endpoint, err := plugins.Find(n)
			if err != nil {
				return nil, err
			}
			return flavor_client.NewClient(n, endpoint.Address)
		}

		groupPlugin := group.NewGroupPlugin(instancePluginLookup, flavorPluginLookup, *pollInterval, *maxParallelNum)

		// Start a poller to load the snapshot and make that available as metadata
		updateSnapshot := make(chan func(map[string]interface{}))
		stopSnapshot := make(chan struct{})
		go func() {
			tick := time.Tick(1 * time.Second)
			tick30 := time.Tick(30 * time.Second)
			for {
				select {
				case <-tick:
					// load the specs for the groups
					snapshot := map[string]interface{}{}
					if specs, err := groupPlugin.InspectGroups(); err == nil {
						for _, spec := range specs {
							snapshot[string(spec.ID)] = spec
						}
					} else {
						snapshot["err"] = err
					}

					updateSnapshot <- func(view map[string]interface{}) {
						metadata_plugin.Put([]string{"specs"}, snapshot, view)
					}

				case <-tick30:
					snapshot := map[string]interface{}{}
					// describe the groups and expose info as metadata
					if specs, err := groupPlugin.InspectGroups(); err == nil {
						for _, spec := range specs {
							if description, err := groupPlugin.DescribeGroup(spec.ID); err == nil {
								snapshot[string(spec.ID)] = description
							} else {
								snapshot[string(spec.ID)] = err
							}
						}
					} else {
						snapshot["err"] = err
					}

					updateSnapshot <- func(view map[string]interface{}) {
						metadata_plugin.Put([]string{"groups"}, snapshot, view)
					}

				case <-stopSnapshot:
					log.Infoln("Snapshot updater stopped")
					return
				}
			}
		}()

		cli.RunPlugin(*name,
			metadata_rpc.PluginServer(metadata_plugin.NewPluginFromChannel(updateSnapshot)),
			group_server.PluginServer(groupPlugin))

		close(stopSnapshot)

		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
