package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit.digitalocean/plugin/instance"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "DigitalOcean instance plugin",
	}
	name := cmd.Flags().String("name", "instance-digitalocean", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	region := cmd.Flags().String("region", "", "DigitalOcean region")
	//config := cmd.Flags().String("config", "$HOME/.config/doctl/config.yaml", "configuration file where the api token are specified")
	token := cmd.Flags().String("access-token", "", "DigitalOcean token")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(instance.NewDOInstancePlugin(*token, *region)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
