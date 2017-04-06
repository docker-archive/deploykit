package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	application "github.com/docker/infrakit/pkg/rpc/application"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Event Repeater Application plugin",
	}
	name := cmd.Flags().String("name", "app-event-repeater", "Application name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	source := cmd.Flags().String("source", "event-plugin", "Event sourve address.")
	sink := cmd.Flags().String("sink", "localhost:1883", "Event sink address. default: localhost:1883")
	sinkProtocol := cmd.Flags().String("sinkprotocol", "mqtt", "Event sink protocol. Now only mqtt and stderr is implemented.")
	allowall := cmd.Flags().Bool("allowall", false, "Allow all event from source and repeat the event to sink as same topic name. default: false")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, application.PluginServer(NewEventRepeater(*source, *sink, *sinkProtocol, *allowall)))
		return nil
	}

	//	cmd.AddCommand(cli.VersionCommand())

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
