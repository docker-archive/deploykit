package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/discovery"
	swarm_leader "github.com/docker/infrakit/leader/swarm"
	swarm_store "github.com/docker/infrakit/store/swarm"
	"github.com/docker/infrakit/util/docker"
	"github.com/spf13/cobra"
)

func swarmEnvironment(backend *backend) *cobra.Command {

	tlsOptions := tlsconfig.Options{}
	host := "unix:///var/run/docker.sock"

	pollInterval := 5 * time.Second

	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "swarm mode for leader detection and storage",
		RunE: func(c *cobra.Command, args []string) error {

			dockerClient, err := docker.NewDockerClient(host, &tlsOptions)
			log.Infoln("Connect to docker", host, "err=", err)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			leader := swarm_leader.NewDetector(pollInterval, dockerClient)
			snapshot, err := swarm_store.NewSnapshot(dockerClient)
			if err != nil {
				return err
			}

			plugins, err := discovery.NewPluginDiscovery()
			if err != nil {
				return err
			}

			backend.plugins = plugins
			backend.leader = leader
			backend.snapshot = snapshot
			return nil
		},
	}

	cmd.Flags().DurationVar(&pollInterval, "poll-interval", pollInterval, "Leader polling interval")
	cmd.Flags().StringVar(&host, "host", host, "Docker host")
	cmd.Flags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert file path")
	cmd.Flags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert file path")
	cmd.Flags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key file path")
	cmd.Flags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")

	return cmd
}
