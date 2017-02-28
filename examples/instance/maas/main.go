package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "MAAS instance plugin",
	}
	name := cmd.Flags().String("name", "instance-maas", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	apiKey := cmd.Flags().String("apikey", "aaaa:bbbb:ccccc", "MAAS API KEY. <consumer_key>:<key>:<secret>")
	maasURL := cmd.Flags().String("url", "127.0.0.1:80", "MAAS Server URL. <url>:<port>")
	apiVersion := cmd.Flags().String("apiversion", "1.0", "MAAS api Version. 1.0")
	defaultDir, err := os.Getwd()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	dir := cmd.Flags().String("dir", defaultDir, "MaaS directory")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(NewMaasPlugin(*dir, *apiKey, *maasURL, *apiVersion)))
		return nil
	}

	//	cmd.AddCommand(cli.VersionCommand())

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
