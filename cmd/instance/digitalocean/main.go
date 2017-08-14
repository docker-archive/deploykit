package main

import (
	"context"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/digitalocean/godo"
	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/provider/digitalocean/plugin/instance"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/run"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func main() {

	var namespaceTags []string

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "DigitalOcean instance plugin",
	}
	name := cmd.Flags().String("name", "instance-digitalocean", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")

	//config := cmd.Flags().String("config", "$HOME/.config/doctl/config.yaml", "configuration file where the api token are specified")
	accessToken := cmd.Flags().String("access-token", "", "DigitalOcean token")

	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		token := &oauth2.Token{AccessToken: *accessToken}
		tokenSource := oauth2.StaticTokenSource(token)
		oauthClient := oauth2.NewClient(context.TODO(), tokenSource)
		client := godo.NewClient(oauthClient)

		namespace := map[string]string{}
		for _, tagKV := range namespaceTags {
			keyAndValue := strings.Split(tagKV, "=")
			if len(keyAndValue) != 2 {
				log.Error("Namespace tags must be formatted as key=value")
				os.Exit(1)
			}

			namespace[keyAndValue[0]] = keyAndValue[1]
		}

		run.Plugin(plugin.DefaultTransport(*name),
			instance_plugin.PluginServer(instance.NewDOInstancePlugin(client, namespace)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
