package main

import (
	"errors"
	"os"

	"github.com/docker/infrakit.gcp/plugin/instance"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "GCE instance plugin",
	}

	name := cmd.Flags().String("name", "instance-gce", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	project := cmd.Flags().String("project", "", "Google Cloud project")
	zone := cmd.Flags().String("zone", "europe-west1-d", "Google Cloud zone")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		if *project == "" {
			return errors.New("Missing project")
		}
		if *zone == "" {
			return errors.New("Missing zone")
		}

		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, instance_plugin.PluginServer(instance.NewGCEInstancePlugin(*project, *zone)))
		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
