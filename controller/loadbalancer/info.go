package loadbalancer

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

// AmISwarmLeader determines if the current node is the swarm manager leader
func AmISwarmLeader(client client.APIClient, ctx context.Context) (bool, error) {
	info, err := client.Info(ctx)

	log.Debugln("info=", info, "err=", err)

	if err != nil {
		return false, err
	}

	// inspect itself to see if i am the leader
	node, _, err := client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)

	log.Debugln("nodeId=", info.Swarm.NodeID, "node=", node, "err=", err)

	if err != nil {
		return false, err
	}

	log.Debugln("manager=", node.ManagerStatus)

	if node.ManagerStatus == nil {
		return false, nil
	}

	log.Debugln("leader=", node.ManagerStatus.Leader)

	return node.ManagerStatus.Leader, nil
}
