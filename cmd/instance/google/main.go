package main

import (
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	google_plugin "github.com/docker/infrakit/pkg/provider/google/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/provider/google/plugin/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/provider/google/plugin/metadata"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "GCE instance plugin",
	}

	name := cmd.Flags().String("name", "instance-gcp", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	project := cmd.Flags().String("project", "", "Google Cloud project")
	zone := cmd.Flags().String("zone", "", "Google Cloud zone")
	namespaceTags := cmd.Flags().StringSlice("namespace-tags", []string{},
		"A list of key=value resource tags to namespace all resources created")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		namespace := map[string]string{}
		for _, tagKV := range *namespaceTags {
			kv := strings.Split(tagKV, "=")
			if len(kv) != 2 {
				log.Errorln("Namespace tags must be formatted as key=value")
				os.Exit(1)
			}
			namespace[kv[0]] = kv[1]
		}

		log.Debug("Using namespace", namespace)

		run.Plugin(plugin.DefaultTransport(*name),
			instance_rpc.PluginServer(instance_plugin.NewGCEInstancePlugin(*project, *zone, namespace)),
			metadata_rpc.PluginServer(metadata_plugin.NewGCEMetadataPlugin(*project, *zone)),
		)
	}

	cmd.AddCommand(google_plugin.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
