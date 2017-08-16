package mux

import (
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/go-connections/tlsconfig"
	etcd_leader "github.com/docker/infrakit/pkg/leader/etcd/v3"
	etcd "github.com/docker/infrakit/pkg/util/etcd/v3"
	"github.com/spf13/cobra"
)

func etcdEnvironment(cfg *config) *cobra.Command {

	defaultEndpoint := etcd.LocalIP() + ":2379"

	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "etcd v3 for leader detection and storage",
	}

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
		logger.Info("Connect to etcd3", "endpoint", *endpoint, "err", err)
		if err != nil {
			return err
		}
		defer etcdClient.Close()

		// Start the leader and storage backends
		cfg.poller = etcd_leader.NewDetector(*cfg.pollInterval, etcdClient)
		cfg.store = etcd_leader.NewStore(etcdClient)

		return runMux(cfg)
	}

	return cmd
}
