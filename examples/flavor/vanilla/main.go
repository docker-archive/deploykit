package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
)

var defaultTemplateOptions = template.Options{MultiPass: true}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Vanilla flavor plugin",
	}
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	name := cmd.Flags().String("name", "flavor-vanilla", "Plugin name to advertise for discovery")
	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, flavor_plugin.PluginServer(NewPlugin()))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
