package manager

import (
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/infrakit/pkg/leader"
	etcd_leader "github.com/docker/infrakit/pkg/leader/etcd/v3"
	"github.com/docker/infrakit/pkg/store"
	etcd_store "github.com/docker/infrakit/pkg/store/etcd/v3"
	"github.com/docker/infrakit/pkg/types"
	etcd "github.com/docker/infrakit/pkg/util/etcd/v3"
)

// BackendEtcdOptions contain the options for the etcd backend
type BackendEtcdOptions struct {
	// PollInterval is how often to check
	PollInterval time.Duration

	etcd.Options `json:",inline" yaml:",inline"`

	// TLS config
	TLS *tlsconfig.Options
}

// DefaultBackendEtcdOptions contains the defaults for running etcd as backend
var DefaultBackendEtcdOptions = Options{
	Backend: "etcd",
	Settings: types.AnyValueMust(
		BackendEtcdOptions{
			PollInterval: 5 * time.Second,
			Options: etcd.Options{
				RequestTimeout: 1 * time.Second,
				Config: clientv3.Config{
					Endpoints: []string{etcd.LocalIP() + ":2379"},
				},
			},
		},
	),
}

func etcdBackends(options BackendEtcdOptions) (leader.Detector, store.Snapshot, cleanup, error) {
	if options.TLS != nil {
		config, err := tlsconfig.Client(*options.TLS)
		if err != nil {
			return nil, nil, nil, err
		}
		options.Options.Config.TLS = config
	}

	etcdClient, err := etcd.NewClient(options.Options)
	log.Info("Connect to etcd3", "endpoint", options.Options.Config.Endpoints, "err", err)
	if err != nil {
		return nil, nil, nil, err
	}

	leader := etcd_leader.NewDetector(options.PollInterval, etcdClient)
	snapshot, err := etcd_store.NewSnapshot(etcdClient)
	if err != nil {
		return nil, nil, nil, err
	}
	return leader, snapshot, func() { etcdClient.Close() }, nil
}
