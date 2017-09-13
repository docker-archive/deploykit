package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/aws/plugin/metadata"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// go run plugin/reflect/cmd/main.go --stack dchung1 --region us-west-1 will reflect on the stack 'dchung1'
func main() {

	options := &metadata.Options{}

	var logLevel int
	var name, stack, templateURL string
	poll := 1 * time.Second

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "AWS metadata plugin",
		RunE: func(c *cobra.Command, args []string) error {

			cli.SetLogLevel(logLevel)

			stop := make(chan struct{})

			metadataPlugin, err := metadata.NewPlugin(metadata.Options{
				Template:        templateURL,
				TemplateOptions: template.Options{},
				PollInterval:    types.Duration(poll),
				StackName:       stack,
			},
				stop)
			if err != nil {
				return err
			}

			run.Plugin(plugin.DefaultTransport(name), metadata_rpc.PluginServer(metadataPlugin))

			close(stop)
			return nil
		},
	}
	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&name, "name", "metadata-aws", "Plugin name to advertise for discovery")
	cmd.Flags().AddFlagSet(options.Flags())
	cmd.Flags().StringVar(&stack, "stack", "", "CFN stack name to introspect")
	cmd.Flags().StringVar(&templateURL, "template-url", "", "URL of the template to evaluate and export metadata.")
	cmd.Flags().DurationVar(&poll, "poll-interval", poll, "Polling interval")

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
