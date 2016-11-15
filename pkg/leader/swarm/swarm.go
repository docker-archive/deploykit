package swarm

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/leader"
	"golang.org/x/net/context"
)

// NewDetector return an implementation of leader detector
func NewDetector(pollInterval time.Duration, client client.APIClient) leader.Detector {
	return leader.NewPoller(pollInterval, func() (bool, error) {
		return amISwarmLeader(context.Background(), client)
	})
}

// amISwarmLeader determines if the current node is the swarm manager leader
func amISwarmLeader(ctx context.Context, client client.APIClient) (bool, error) {
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
	log.Debugln("leader=", node.ManagerStatus.Leader, "node=", node)
	return node.ManagerStatus.Leader, nil
}
