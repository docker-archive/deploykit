package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/discovery/local"
	swarm_leader "github.com/docker/infrakit/pkg/leader/swarm"
	swarm_store "github.com/docker/infrakit/pkg/store/swarm"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/spf13/cobra"
)

func swarmEnvironment(getConfig func() config) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "swarm mode for leader detection and storage",
	}
	pollInterval := cmd.Flags().Duration("poll-interval", 5*time.Second, "Leader polling interval")
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
		log.Infoln("Connect to docker", host, "err=", err)
		if err != nil {
			return err
		}
		defer dockerClient.Close()

		leader := swarm_leader.NewDetector(*pollInterval, dockerClient)
		leaderStore := swarm_leader.NewStore(dockerClient)
		snapshot, err := swarm_store.NewSnapshot(dockerClient)
		if err != nil {
			return err
		}

		plugins, err := local.NewPluginDiscovery()
		if err != nil {
			return err
		}

		cfg := getConfig()
		cfg.plugins = plugins
		cfg.leader = leader
		cfg.leaderStore = leaderStore
		cfg.snapshot = snapshot

		return runMain(cfg)
	}

	return cmd
}
