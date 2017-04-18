package main

import (
	"context"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/digitalocean/godo"
	"github.com/docker/infrakit.digitalocean/plugin/instance"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
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
	accessToken := cmd.Flags().String("access-token", "", "DigitalOcean token")
	sshKey := cmd.Flags().String("sshKey", "", "Default ssh key to use for droplets (it has to exists on digitalocean)")

	cmd.Run = func(c *cobra.Command, args []string) {
		cli.SetLogLevel(*logLevel)

		token := &oauth2.Token{AccessToken: *accessToken}
		tokenSource := oauth2.StaticTokenSource(token)
		oauthClient := oauth2.NewClient(context.TODO(), tokenSource)
		client := godo.NewClient(oauthClient)

		cli.RunPlugin(*name, instance_plugin.PluginServer(instance.NewDOInstancePlugin(client, *region, *sshKey)))
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
