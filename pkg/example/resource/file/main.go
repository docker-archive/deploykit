package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "File resource plugin",
	}
	name := cmd.Flags().String("name", "resource-file", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	dir := cmd.Flags().String("dir", os.TempDir(), "Dir for storing the files")
	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)
		cli.RunPlugin(*name, resource.PluginServer(NewFileResourcePlugin(*dir)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
