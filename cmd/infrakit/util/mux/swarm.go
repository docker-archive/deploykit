package mux

import (
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/leader/swarm"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/spf13/cobra"
)

func swarmEnvironment(cfg *config) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "swarm mode for leader detection and storage",
	}

	host := cmd.Flags().String("host", "unix:///var/run/docker.sock", "Docker host")
	caFile := cmd.Flags().String("tlscacert", "", "TLS CA cert file path")
	certFile := cmd.Flags().String("tlscert", "", "TLS cert file path")
	tlsKey := cmd.Flags().String("tlskey", "", "TLS key file path")
	insecureSkipVerify := cmd.Flags().Bool("tlsverify", true, "True to skip TLS")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		dockerClient, err := docker.NewClient(*host, &tlsconfig.Options{
			CAFile:             *caFile,
			CertFile:           *certFile,
			KeyFile:            *tlsKey,
			InsecureSkipVerify: *insecureSkipVerify,
		})
		logger.Info("Connect to docker", "host", host, "err", err)
		if err != nil {
			return err
		}
		defer dockerClient.Close()

		cfg.poller = swarm.NewDetector(*cfg.pollInterval, dockerClient)
		cfg.store = swarm.NewStore(dockerClient)

		return runMux(cfg)
	}

	return cmd
}
