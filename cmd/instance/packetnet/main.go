package main

import (
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin/instance/packetnet"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "plugin/instance/packetnet")

func main() {

	var namespaceTags []string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Packetnet instance plugin",
	}
	name := cmd.Flags().String("name", "instance-packnet", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	apiToken := cmd.Flags().String("access-token", "", "API token")
	projectID := cmd.Flags().String("project-id", "", "Project ID")

	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		namespace := map[string]string{}
		for _, tagKV := range namespaceTags {
			keyAndValue := strings.Split(tagKV, "=")
			if len(keyAndValue) != 2 {
				log.Error("Namespace tags must be formatted as key=value")
				os.Exit(1)
			}

			namespace[keyAndValue[0]] = keyAndValue[1]
		}

		cli.RunPlugin(*name, instance_plugin.PluginServer(packetnet.NewPlugin(*projectID, *apiToken, namespace)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Crit("error", "err", err)
		os.Exit(1)
	}
}
