package etcd

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
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
func AmILeader(ctx context.Context, client *etcd.Client) (isLeader bool, err error) {

	endpoint := ""
	var statusResp *clientv3.StatusResponse

	defer func() {
		log.V(100).Infoln("checking status at", endpoint, "resp=", statusResp, "err=", err, "leader=", isLeader)
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
