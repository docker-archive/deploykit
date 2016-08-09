package swarm

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
	"strings"
	"time"
)

func isLeading(ctx context.Context, client client.APIClient) (bool, error) {
	info, err := client.Info(ctx)
	log.Debugln("info=", info, "err=", err)
	if err != nil {
		return false, err
	}

	node, _, err := client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	log.Debugln("node=", node, "err=", err)
	if err != nil {
		if strings.Contains(err.Error(), "This node is not a Swarm manager") {
			// The engine returns an error for this request when the node is not a manager, so
			// we handle this error specially and treat it as non-leading.
			return false, nil
		}

		return false, err
	}

	if node.ManagerStatus == nil {
		return false, nil
	}

	return node.ManagerStatus.Leader, nil
}

// RunWhenLeading polls the Docker Engine to determine whether it is a swarm leader.  On a positive leading edge
// (first observance of leading state), `leadingStart` will be invoked.  On a negative leading edge (transition from
// leading to non-leading or unknown state), `leadingEnd` will be invoked.
// It is the reponsibility of the caller to avoid blocking the poll routine when callbacks are invoked.
func RunWhenLeading(
	ctx context.Context,
	client client.APIClient,
	pollInterval time.Duration,
	leadingStart func(),
	leadingEnd func()) error {

	tick := time.Tick(pollInterval)
	wasLeader := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick:
			isLeader, err := isLeading(ctx, client)
			if err != nil {
				if wasLeader {
					leadingEnd()
				}

				return err
			}

			if isLeader && !wasLeader {
				leadingStart()
			} else if !isLeader && wasLeader {
				leadingEnd()
				return nil
			}
			wasLeader = isLeader
		}
	}
}
