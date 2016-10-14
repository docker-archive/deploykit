package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/cli"
	"github.com/docker/infrakit/plugin/instance/vagrant"
	instance_plugin "github.com/docker/infrakit/spi/http/instance"
	"github.com/spf13/cobra"
	"os"
)

func main() {

	logLevel := cli.DefaultLogLevel
	name := "instance-vagrant"
	dir, _ := os.Getwd()

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Vagrant instance plugin",
		Run: func(c *cobra.Command, args []string) {

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, instance_plugin.PluginServer(vagrant.NewVagrantPlugin(dir)))
		},
	}

	cmd.AddCommand(cli.VersionCommand())

	cmd.Flags().String("name", name, "Plugin name to advertise for discovery")
	cmd.Flags().IntVar(&logLevel, "log", logLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&dir, "dir", dir, "Vagrant directory")

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
