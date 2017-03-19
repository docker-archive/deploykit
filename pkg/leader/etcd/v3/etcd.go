package etcd

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/leader"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	log "github.com/golang/glog"
	"golang.org/x/net/context"
)

// NewDetector return an implementation of leader detector
func NewDetector(pollInterval time.Duration, client *etcd.Client) leader.Detector {
	return leader.NewPoller(pollInterval, func() (bool, error) {
		return AmILeader(context.Background(), client)
	})
}

// AmILeader checks if this node is a leader
func AmILeader(ctx context.Context, client *etcd.Client) (bool, error) {

	// get status of node
	endpoint := ""
	if len(client.Options.Config.Endpoints) > 0 {
		endpoint = client.Options.Config.Endpoints[0]
	}

	if endpoint == "" {
		return false, fmt.Errorf("bad config:%v", client.Options)
	}

	statusResp, err := client.Client.Status(ctx, endpoint)
	log.V(50).Infoln("checking status at", endpoint, "resp=", statusResp, "err=", err)
	if err != nil {
		return false, err
	}

	// The header has the self, assuming the endpoint is the self node.
	// The response has the id of the leader. So just compare self id and the leader id.
	return statusResp.Leader == statusResp.Header.MemberId, nil
}
