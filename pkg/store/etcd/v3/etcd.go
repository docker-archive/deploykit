package etcd

import (
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	log "github.com/golang/glog"
	"golang.org/x/net/context"
)

const (

	// DefaultKey is the key used to persist the config.
	DefaultKey = "infrakit/configs/groups.json"
)

// NewSnapshot returns a snapshot given the options
func NewSnapshot(options etcd.Options) (store.Snapshot, error) {
	cli, err := etcd.NewClient(options)
	if err != nil {
		return nil, err
	}
	return &snapshot{
		client: cli,
		key:    DefaultKey,
	}, nil
}

type snapshot struct {
	client *etcd.Client
	key    string
}

// Save marshals (encodes) and saves a snapshot of the given object.
func (s *snapshot) Save(obj interface{}) error {

	any, err := types.AnyValue(obj)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.client.Options.RequestTimeout)
	_, err = s.client.Client.Put(ctx, s.key, any.String())
	cancel()
	if err != nil {
		switch err {
		case context.Canceled:
			log.Warningf("ctx is canceled by another routine: %v", err)
		case context.DeadlineExceeded:
			log.Warningf("ctx is attached with a deadline is exceeded: %v", err)
		case rpctypes.ErrEmptyKey:
			log.Warningf("client-side error: %v", err)
		default:
			log.Warningf("bad cluster endpoints, which are not etcd servers: %v", err)
		}
	}
	return err
}

// Load loads a snapshot and marshals (decodes) into the given reference.
// If no data is available to unmarshal into the given struct, the fuction returns nil.
func (s *snapshot) Load(output interface{}) error {

	ctx, cancel := context.WithTimeout(context.Background(), s.client.Options.RequestTimeout)
	resp, err := s.client.Client.Get(ctx, s.key)
	cancel()
	if err != nil {
		switch err {
		case context.Canceled:
			log.Warningf("ctx is canceled by another routine: %v", err)
		case context.DeadlineExceeded:
			log.Warningf("ctx is attached with a deadline is exceeded: %v", err)
		case rpctypes.ErrEmptyKey:
			log.Warningf("client-side error: %v", err)
		default:
			log.Warningf("bad cluster endpoints, which are not etcd servers: %v", err)
		}
	}

	if resp.Count > 1 {
		log.Warningf("more than 1 config %v", resp)
		return nil
	}

	pair := resp.Kvs[0]
	any := types.AnyBytes(pair.Value)
	return any.Decode(&output)
}

// Close releases the resources and closes the connection to etcd
func (s *snapshot) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}
