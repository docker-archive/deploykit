package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	zk "github.com/docker/infrakit/plugin/flavor/zookeeper"
	flavor_plugin "github.com/docker/infrakit/spi/http/flavor"
	"github.com/spf13/cobra"
	"os"
)

func main() {

	logLevel := cli.DefaultLogLevel
	name := "flavor-zooker"

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Zookeeper flavor plugin",
		Run: func(c *cobra.Command, args []string) {

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, flavor_plugin.PluginServer(zk.NewPlugin()))
		},
	}

	cmd.AddCommand(cli.VersionCommand())

	cmd.Flags().String("name", name, "Plugin name to advertise for discovery")
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
