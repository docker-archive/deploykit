package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/run/flavor/vanilla"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var defaultTemplateOptions = template.Options{MultiPass: true}

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Vanilla flavor plugin",
	}

	options := vanilla.DefaultOptions

	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	cmd.Flags().StringVar(&options.Name, "name", options.Name, "Plugin name to advertise for discovery")
	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		name, impl, onStop, err := vanilla.Run(nil, types.AnyValueMust(options))
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
