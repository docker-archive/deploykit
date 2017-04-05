package main

import (
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.gcp/plugin"
	"github.com/docker/infrakit.gcp/plugin/instance"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
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

		cli.RunPlugin(*name, instance_plugin.PluginServer(instance.NewGCEInstancePlugin(*project, *zone, namespace)))
	}

	cmd.AddCommand(plugin.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
