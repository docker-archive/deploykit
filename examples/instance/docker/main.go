package main

import (
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func main() {

	builder := Builder{}

	var logLevel int
	var name string
	var namespaceTags []string
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Docker instance plugin",
		Run: func(c *cobra.Command, args []string) {

			namespace := map[string]string{}
			for _, tagKV := range namespaceTags {
				keyAndValue := strings.Split(tagKV, "=")
				if len(keyAndValue) != 2 {
					log.Error("Namespace tags must be formatted as key=value")
					os.Exit(1)
				}

				namespace[keyAndValue[0]] = keyAndValue[1]
			}
			// channel for metadata update
			updateSnapshot := make(chan func(map[string]interface{}))
			// channel for metadata update stop signal
			stopSnapshot := make(chan struct{})
			instancePlugin, err := builder.BuildInstancePlugin(namespace)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			// filter on containers managed by InfraKit
			filter := filters.NewArgs()
			filter.Add("label", "infrakit.group")
			options := apitypes.ContainerListOptions{Filters: filter}
			go func() {
				tick := time.Tick(2 * time.Second)
				for {
					select {
					case <-tick:
						snapshot := map[string]interface{}{}
						// list all the containers, inspect them and add it to the snapshot
						ctx := context.Background()
						containers, err := builder.client.ContainerList(ctx, options)
						if err != nil {
							log.Warnln("Metadata update failed to list containers")
							snapshot["err"] = err
							continue
						}
						for _, container := range containers {
							cid := container.ID
							if json, err := builder.client.ContainerInspect(ctx, cid); err == nil {
								snapshot[cid] = json
							} else {
								log.Warnln("Failed to get metadata for container %s", cid)
								snapshot["err"] = err
								log.Error(err)
							}
						}
						updateSnapshot <- func(view map[string]interface{}) {
							types.Put([]string{"containers"}, snapshot, view)
						}

					case <-stopSnapshot:
						log.Infoln("Snapshot updater stopped")
						return
					}
				}
			}()

			cli.SetLogLevel(logLevel)
			run.Plugin(plugin.DefaultTransport(name),
				metadata_rpc.PluginServer(metadata.NewPluginFromChannel(updateSnapshot)),
				instance_plugin.PluginServer(instancePlugin),
			)

			close(stopSnapshot)
		},
	}

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&name, "name", "instance-docker", "Plugin name to advertise for discovery")
	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	cmd.Flags().AddFlagSet(builder.Flags())

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
