package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	rackhd "github.com/docker/infrakit/pkg/provider/rackhd/plugin"
	"github.com/docker/infrakit/pkg/provider/rackhd/plugin/instance"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/run"
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
			run.Plugin(plugin.DefaultTransport(name), instance_plugin.PluginServer(instancePlugin))
		},
	}

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging Level. 0 is the least verbose. Max is 5.")
	cmd.Flags().StringVar(&name, "name", "rackhd", "Plugin name to advertise for discovery")
	cmd.Flags().AddFlagSet(builder.Flags())

	cmd.AddCommand(rackhd.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
