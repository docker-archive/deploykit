package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/v0/file"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "File instance plugin",
	}

	options := file.DefaultOptions
	cmd.Flags().StringVar(&options.Name, "name", options.Name, "Plugin name to advertise for discovery")
	cmd.Flags().StringVar(&options.Dir, "dir", options.Dir, "Directory path to store the files")

	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		name, impl, onStop, err := file.Run(nil, types.AnyValueMust(options))
		if err != nil {
			return
		}

		_, running, err := run.ServeRPC(name, onStop, impl)
		if err != nil {
			return
		}
		<-running
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
