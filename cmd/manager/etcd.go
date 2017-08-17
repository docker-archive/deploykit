package main

import (
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/discovery/local"
	etcd_leader "github.com/docker/infrakit/pkg/leader/etcd/v3"
	etcd_store "github.com/docker/infrakit/pkg/store/etcd/v3"
	etcd "github.com/docker/infrakit/pkg/util/etcd/v3"
	log "github.com/golang/glog"
	"github.com/spf13/cobra"
)

func etcdEnvironment(getConfig func() config) *cobra.Command {

	defaultEndpoint := etcd.LocalIP() + ":2379"

	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "etcd v3 for leader detection and storage",
	}
	pollInterval := cmd.Flags().Duration("poll-interval", 5*time.Second, "Leader polling interval")
	requestTimeout := cmd.Flags().Duration("request-timeout", 1*time.Second, "Request timeout")
	endpoint := cmd.Flags().String("endpoint", defaultEndpoint, "Etcd endpoint (v3 grpc)")
	caFile := cmd.Flags().String("tlscacert", "", "TLS CA cert file path")
	certFile := cmd.Flags().String("tlscert", "", "TLS cert file path")
	tlsKey := cmd.Flags().String("tlskey", "", "TLS key file path")
	insecureSkipVerify := cmd.Flags().Bool("tlsverify", true, "True to skip TLS")
	cmd.RunE = func(c *cobra.Command, args []string) error {

		options := etcd.Options{
			Config: clientv3.Config{
				Endpoints: []string{*endpoint},
			},
			RequestTimeout: *requestTimeout,
		}

		if *caFile != "" && *certFile != "" && *tlsKey != "" {
			config, err := tlsconfig.Client(tlsconfig.Options{
				CAFile:             *caFile,
				CertFile:           *certFile,
				KeyFile:            *tlsKey,
				InsecureSkipVerify: *insecureSkipVerify,
			})

			if err != nil {
				return err
			}
			options.Config.TLS = config
		}

		etcdClient, err := etcd.NewClient(options)
		log.Infoln("Connect to etcd3", *endpoint, "err=", err)
		if err != nil {
			return err
		}
		defer etcdClient.Close()

		// Start the leader and storage backends

		leader := etcd_leader.NewDetector(*pollInterval, etcdClient)
		leaderStore := etcd_leader.NewStore(etcdClient)
		snapshot, err := etcd_store.NewSnapshot(etcdClient)
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
