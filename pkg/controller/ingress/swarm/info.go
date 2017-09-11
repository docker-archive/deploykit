package swarm

import (
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

// AmISwarmLeader determines if the current node is the swarm manager leader
func AmISwarmLeader(ctx context.Context, client client.APIClient) (bool, error) {
	info, err := client.Info(ctx)

	log.Debug("Swarm info", "info", info, "err", err)

	if err != nil {
		return false, err
	}

	// inspect itself to see if i am the leader
	node, _, err := client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)

	log.Debug("Inspect node", "nodeID", info.Swarm.NodeID, "node", node, "err", err, "V", debugV)

	if err != nil {
		return false, err
	}

	log.Debug("manager status", "status", node.ManagerStatus, "V", debugV)

	if node.ManagerStatus == nil {
		return false, nil
	}

	log.Debug("leader status", "leader", node.ManagerStatus.Leader)

	return node.ManagerStatus.Leader, nil
}
