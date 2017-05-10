package main

import (
	"flag"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	discovery_local "github.com/docker/infrakit/pkg/discovery/local"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin/instance/image"
	"github.com/docker/infrakit/pkg/plugin/metadata"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	metadata_plugin "github.com/docker/infrakit/pkg/rpc/metadata"
	instance_spi "github.com/docker/infrakit/pkg/spi/instance"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "instance/image")

func init() {
	logutil.Configure(&logutil.ProdDefaults)
}

func main() {

	// Log setup
	logOptions := &logutil.ProdDefaults

	if err := discovery_local.Setup(); err != nil {
		panic(err)
	}

	var namespaceTags []string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Image instance plugin <config-url>",
	}

	name := cmd.PersistentFlags().String("name", "instance-image", "Plugin name to advertise for discovery")
	logLevel := cmd.PersistentFlags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.PersistentFlags().AddFlagSet(cli.Flags(logOptions))
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		logutil.Configure(logOptions)
		return nil
	}

	// RUN -------------------------------------
	run := &cobra.Command{
		Use:   "run",
		Short: "Run the plugin",
	}
	run.RunE = func(c *cobra.Command, args []string) error {

		if len(args) != 0 {
			cmd.Usage()
			os.Exit(-1)
		}

		cli.SetLogLevel(*logLevel)

		namespace := map[string]string{}
		for _, tagKV := range namespaceTags {
			keyAndValue := strings.Split(tagKV, "=")
			if len(keyAndValue) != 2 {
				log.Error("Namespace tags must be formatted as key=value")
				os.Exit(1)
			}

			namespace[keyAndValue[0]] = keyAndValue[1]
		}

		imagePlugin, err := image.NewPlugin(namespace)
		if err != nil {
			return err
		}

		cli.RunPlugin(*name,
			instance_plugin.PluginServer(imagePlugin),
			metadata_plugin.PluginServer(metadata.NewPluginFromData(
				map[string]interface{}{
					"version":    cli.Version,
					"revision":   cli.Revision,
					"implements": instance_spi.InterfaceSpec,
				},
			)),
		)
		return nil
	}

	cmd.AddCommand(cli.VersionCommand(), run)

	if err := cmd.Execute(); err != nil {
		log.Crit("Error", "err", err)
		os.Exit(1)
	}
}
