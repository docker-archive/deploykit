package etcd

import (
	"path"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	"golang.org/x/net/context"
)

const (
	namespace = "infrakit/configs"
)

var log = logutil.New("module", "etcd/store")

// NewSnapshot returns a snapshot given the client
func NewSnapshot(client *etcd.Client, key string) (store.Snapshot, error) {
	return &snapshot{
		client: client,
		key:    path.Join(namespace, key),
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
			log.Warn("ctx is canceled by another routine", "err", err)
		case context.DeadlineExceeded:
			log.Warn("ctx is attached with a deadline is exceeded", "err", err)
		case rpctypes.ErrEmptyKey:
			log.Warn("client-side error", "err", err)
		default:
			log.Warn("bad cluster endpoints, which are not etcd servers", "err", err)
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
			log.Warn("ctx is canceled by another routine", "err", err)
		case context.DeadlineExceeded:
			log.Warn("ctx is attached with a deadline is exceeded", "err", err)
		case rpctypes.ErrEmptyKey:
			log.Warn("client-side error", "err", err)
		default:
			log.Warn("bad cluster endpoints, which are not etcd servers", "err", err)
		}
	}

	if resp == nil {
		log.Warn("response is nil. server down?")
		return nil
	}

	if resp.Count > 1 {
		log.Warn("more than 1 config", "resp", resp)
		return nil
	}

	if resp.Count == 0 {
		// no data. therefore no effect on the input
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
