package etcd

import (
	"fmt"
	"net/url"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	"golang.org/x/net/context"
)

var log = logutil.New("module", "etcd/leader")

// NewDetector return an implementation of leader detector
func NewDetector(pollInterval time.Duration, client *etcd.Client) *leader.Poller {
	return leader.NewPoller(pollInterval, func() (bool, error) {
		return AmILeader(context.Background(), client)
	})
}

// AmILeader checks if this node is a leader
func AmILeader(ctx context.Context, client *etcd.Client) (isLeader bool, err error) {

	endpoint := ""
	var statusResp *clientv3.StatusResponse

	defer func() {
		log.Debug("checking status", "endpoint", endpoint, "resp", statusResp, "err", err, "leader", isLeader)
	}()

	// get status of node
	if len(client.Options.Config.Endpoints) > 0 {
		endpoint = client.Options.Config.Endpoints[0]
	}

	if endpoint == "" {
		isLeader = false
		err = fmt.Errorf("bad config:%v", client.Options)
		return
	}

	statusResp, err = client.Client.Status(ctx, endpoint)
	if err != nil {
		isLeader = false
		return
	}

	// The header has the self, assuming the endpoint is the self node.
	// The response has the id of the leader. So just compare self id and the leader id.
	isLeader = statusResp.Leader == statusResp.Header.MemberId

	return
}

// Store uses ectd as the backend for registration of leader location
type Store struct {
	client *etcd.Client
}

// NewStore returns a store for registration of leader location
func NewStore(c *etcd.Client) leader.Store {
	return &Store{client: c}
}

const (

	// DefaultKey is the key used to persist the location
	DefaultKey = "infrakit/leader/location"
)

// UpdateLocation writes the location to etcd.
func (s Store) UpdateLocation(location *url.URL) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Options.RequestTimeout)
	_, err := s.client.Client.Put(ctx, DefaultKey, location.String())
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

// GetLocation returns the location of the leader
func (s Store) GetLocation() (*url.URL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Options.RequestTimeout)
	resp, err := s.client.Client.Get(ctx, DefaultKey)
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
		return nil, nil
	}

	if resp.Count > 1 {
		log.Warn("more than 1 location", "resp", resp)
		return nil, nil
	}

	if resp.Count == 0 {
		// no data. therefore no effect on the input
		return nil, nil
	}

	pair := resp.Kvs[0]
	return url.Parse(string(pair.Value))
}
