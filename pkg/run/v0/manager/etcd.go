package manager

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/go-connections/tlsconfig"
	etcd_leader "github.com/docker/infrakit/pkg/leader/etcd/v3"
	etcd_store "github.com/docker/infrakit/pkg/store/etcd/v3"
	"github.com/docker/infrakit/pkg/types"
	etcd "github.com/docker/infrakit/pkg/util/etcd/v3"
)

// BackendEtcdOptions contain the options for the etcd backend
type BackendEtcdOptions struct {
	// PollInterval is how often to check
	PollInterval types.Duration

	etcd.Options `json:",inline" yaml:",inline"`

	// TLS config
	TLS *tlsconfig.Options
}

// DefaultBackendEtcdOptions contains the defaults for running etcd as backend
var DefaultBackendEtcdOptions = BackendEtcdOptions{
	PollInterval: types.FromDuration(5 * time.Second),
	Options: etcd.Options{
		RequestTimeout: 1 * time.Second,
		Config: clientv3.Config{
			Endpoints: []string{etcd.LocalIP() + ":2379"},
		},
	},
}

func configEtcdBackends(options BackendEtcdOptions, managerConfig *Options) error {
	if managerConfig != nil {
		return nil
	}

	if options.TLS != nil {
		config, err := tlsconfig.Client(*options.TLS)
		if err != nil {
			return err
		}
		options.Options.Config.TLS = config
	}

	etcdClient, err := etcd.NewClient(options.Options)
	log.Info("Connect to etcd3", "endpoint", options.Options.Config.Endpoints, "err", err)
	if err != nil {
		return err
	}

	leader := etcd_leader.NewDetector(options.PollInterval.Duration(), etcdClient)
	leaderStore := etcd_leader.NewStore(etcdClient)
	snapshot, err := etcd_store.NewSnapshot(etcdClient, "specs")
	if err != nil {
		return err
	}

	managerConfig.Leader = leader
	managerConfig.LeaderStore = leaderStore
	managerConfig.SpecStore = snapshot
	managerConfig.cleanUpFunc = func() { etcdClient.Close() }

	key := "global.vars"
	if !managerConfig.Metadata.IsEmpty() {
		key = fmt.Sprintf("%s.vars", managerConfig.Metadata.Lookup())
	}

	metadataSnapshot, err := etcd_store.NewSnapshot(etcdClient, key)
	if err != nil {
		return err
	}
	managerConfig.MetadataStore = metadataSnapshot
	return nil
}
