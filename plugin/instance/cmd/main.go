package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/codedellemc/infrakit.rackhd/plugin"
	"github.com/codedellemc/infrakit.rackhd/plugin/instance"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {
	builder := &instance.Builder{}

	var logLevel int
	var name string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "RackHD instance plugin",
		Run: func(c *cobra.Command, args []string) {
			instancePlugin, err := builder.BuildInstancePlugin()
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, instance_plugin.PluginServer(instancePlugin))
		},
	}

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging Level. 0 is the least verbose. Max is 5.")
	cmd.Flags().StringVar(&name, "name", "instance-rackhd", "Plugin name to advertise for discovery")
	cmd.Flags().AddFlagSet(builder.Flags())

	cmd.AddCommand(plugin.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
