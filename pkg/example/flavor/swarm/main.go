package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/cli"
	flavor_plugin "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/util/docker/1.24"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Docker Swarm flavor plugin",
	}
	name := cmd.Flags().String("name", "flavor-swarm", "Plugin name to advertise for discovery")
	logLevel := cmd.Flags().Int("log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	host := cmd.Flags().String("host", "unix:///var/run/docker.sock", "Docker host")
	caFile := cmd.Flags().String("tlscacert", "", "TLS CA cert file path")
	certFile := cmd.Flags().String("tlscert", "", "TLS cert file path")
	tlsKey := cmd.Flags().String("tlskey", "", "TLS key file path")
	insecureSkipVerify := cmd.Flags().Bool("tlsverify", true, "True to skip TLS")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		cli.SetLogLevel(*logLevel)

		dockerClient, err := docker.NewDockerClient(*host, &tlsconfig.Options{
			CAFile:             *caFile,
			CertFile:           *certFile,
			KeyFile:            *tlsKey,
			InsecureSkipVerify: *insecureSkipVerify,
		})
		log.Infoln("Connect to docker", host, "err=", err)
		if err != nil {
			return err
		}

		cli.RunPlugin(*name, flavor_plugin.PluginServer(NewSwarmFlavor(dockerClient)))
		return nil
	}

	cmd.AddCommand(cli.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
