package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/discovery"
	swarm_leader "github.com/docker/infrakit/pkg/leader/swarm"
	swarm_store "github.com/docker/infrakit/pkg/store/swarm"
	"github.com/docker/infrakit/pkg/util/docker/1.24"
	"github.com/spf13/cobra"
)

func swarmEnvironment(backend *backend) *cobra.Command {

	tlsOptions := tlsconfig.Options{}
	host := "unix:///var/run/docker.sock"

	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "swarm mode for leader detection and storage",
		RunE: func(c *cobra.Command, args []string) error {

			dockerClient, err := docker.NewDockerClient(host, &tlsOptions)
			log.Infoln("Connect to docker", host, "err=", err)
			if err != nil {
				return err
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

			return runMain(backend)
		},
	}

	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "Leader polling interval")
	cmd.Flags().StringVar(&host, "host", host, "Docker host")
	cmd.Flags().StringVar(&tlsOptions.CAFile, "tlscacert", "", "TLS CA cert file path")
	cmd.Flags().StringVar(&tlsOptions.CertFile, "tlscert", "", "TLS cert file path")
	cmd.Flags().StringVar(&tlsOptions.KeyFile, "tlskey", "", "TLS key file path")
	cmd.Flags().BoolVar(&tlsOptions.InsecureSkipVerify, "tlsverify", true, "True to skip TLS")

	return cmd
}
