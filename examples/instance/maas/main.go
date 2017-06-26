package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/rpc/metadata"
	instance_spi "github.com/docker/infrakit/pkg/spi/instance"
	metadata_spi "github.com/docker/infrakit/pkg/spi/metadata"
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
	apiVersion := cmd.Flags().String("apiversion", "2.0", "MAAS api Version. 2.0")
	if *apiVersion == "1.0" {
		log.Error("MAAS API version 1.0 is no longer supported. You should use 2.0.")
		os.Exit(1)
	}
	defaultDir, err := os.Getwd()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	dir := cmd.Flags().String("dir", defaultDir, "MaaS directory")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		cli.SetLogLevel(*logLevel)

		cli.RunPlugin(*name,
			metadata_plugin.PluginServer(metadata.NewPluginFromData(map[string]interface{}{
				"version":    cli.Version,
				"revision":   cli.Revision,
				"implements": instance_spi.InterfaceSpec,
			})).WithTypes(
				map[string]metadata_spi.Plugin{}),
			instance_plugin.PluginServer(NewMaasPlugin(*dir, *apiKey, *maasURL, *apiVersion)))
		return nil
	}

	//	cmd.AddCommand(cli.VersionCommand())

	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
