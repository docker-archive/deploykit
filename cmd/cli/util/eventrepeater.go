package util

import (
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	eventrepeater "github.com/docker/infrakit/pkg/plugin/application/eventrepeater"
	application "github.com/docker/infrakit/pkg/rpc/application"
	"github.com/spf13/cobra"
)

func eventrepeaterCommand(plugins func() discovery.Plugins) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-repeater",
		Short: "Event Repeater service",
	}

	name := cmd.Flags().String("name", "app-event-repeater", "Application name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	source := cmd.Flags().String("source", "event-plugin", "Event sourve address.")
	sink := cmd.Flags().String("sink", "localhost:1883", "Event sink address. default: localhost:1883")
	sinkProtocol := cmd.Flags().String("sinkprotocol", "mqtt", "Event sink protocol. Now only mqtt and stderr is implemented.")
	allowall := cmd.Flags().Bool("allowall", false, "Allow all event from source and repeat the event to sink as same topic name. default: false")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, application.PluginServer(eventrepeater.NewEventRepeater(*source, *sink, *sinkProtocol, *allowall)))
		return nil
	}
	return cmd
}
