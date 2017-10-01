package swarm

import (
	"net/url"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/leader"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

var (
	log    = logutil.New("module", "leader/swarm")
	debugV = logutil.V(1000)
)

// NewDetector return an implementation of leader detector
func NewDetector(pollInterval time.Duration, client docker.APIClientCloser) *leader.Poller {
	return leader.NewPoller(pollInterval, func() (bool, error) {
		return amISwarmLeader(context.Background(), client)
	})
}

// amISwarmLeader determines if the current node is the swarm manager leader
func amISwarmLeader(ctx context.Context, client docker.APIClientCloser) (bool, error) {
	info, err := client.Info(ctx)

	if err != nil {
		return false, err
	}

	// inspect itself to see if i am the leader
	node, _, err := client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		return false, err
	}

	if node.ManagerStatus == nil {
		return false, nil
	}
	log.Debug("Leadership", "leader", node.ManagerStatus.Leader, "node", node, "V", debugV)
	return node.ManagerStatus.Leader, nil
}

// Store is the backend for storing leader location
type Store struct {
	client docker.APIClientCloser
}

// NewStore constructs a store
func NewStore(c docker.APIClientCloser) leader.Store {
	return &Store{client: c}
}

const (
	// SwarmLabel is the label for the swarm annotation that stores the location of the leader
	SwarmLabel = "infrakit.leader.location"
)

// UpdateLocation writes the location to etcd.
func (s Store) UpdateLocation(location *url.URL) error {
	info, err := s.client.SwarmInspect(context.Background())
	if err != nil {
		return err
	}
	if info.ClusterInfo.Spec.Annotations.Labels == nil {
		info.ClusterInfo.Spec.Annotations.Labels = map[string]string{}
	}
	info.ClusterInfo.Spec.Annotations.Labels[SwarmLabel] = location.String()
	return s.client.SwarmUpdate(context.Background(), info.ClusterInfo.Meta.Version, info.ClusterInfo.Spec,
		swarm.UpdateFlags{})
}

// GetLocation returns the location of the leader
func (s Store) GetLocation() (*url.URL, error) {
	info, err := s.client.SwarmInspect(context.Background())
	if err != nil {
		return nil, err
	}

	if info.ClusterInfo.Spec.Annotations.Labels != nil {
		if l, has := info.ClusterInfo.Spec.Annotations.Labels[SwarmLabel]; has {
			log.Debug("leader location", "location", l, "V", debugV)
			return url.Parse(l)
		}
	}
	return nil, nil
}
