package etcd

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

// Test verifies that the types here are workable with types.Any
func TestTypeAnyable(t *testing.T) {
	options := Options{
		Config: clientv3.Config{
			Endpoints: []string{LocalIP() + ":2379"},
		},
		RequestTimeout: 1 * time.Second,
	}

	any := types.AnyValueMust(options)
	require.True(t, len(any.String()) > 0)

	opt2 := Options{}
	require.NoError(t, any.Decode(&opt2))

	require.Equal(t, options, opt2)
}

func TestRunInContainer(t *testing.T) {

	if testutil.SkipTests("etcd") {
		t.SkipNow()
	}

	ip := LocalIP()
	containerName := "etcd0"

	err := RunContainer.Start(ip, containerName)
	require.NoError(t, err)

	// loop a little until we are ready
	for {
		<-time.After(1 * time.Second)
		output, err := LsMembers.Output(ip)
		log.Infoln("output=", string(output), "err=", err)
		if err == nil {
			log.Infoln("etcd running")
			break
		}
	}

	log.Infoln("Checking node status")

	endpoint := LocalIP() + ":2379"
	client, err := NewClient(Options{
		Config: clientv3.Config{
			Endpoints: []string{endpoint},
		},
		RequestTimeout: 1 * time.Second,
	})

	require.NoError(t, err)

	ctx := context.Background()
	status, err := client.Client.Status(ctx, endpoint)
	require.NoError(t, err)

	self := status.Header.MemberId

	log.Infoln("leader id=", status.Leader, "self id=", self)

	// should be leader since this is a single node cluster
	require.True(t, status.Leader > 0)

	members, err := client.Client.MemberList(ctx)
	require.NoError(t, err)

	for _, m := range members.Members {
		if m.ID == status.Leader {
			log.Infoln("found the leader:", m.Name)

			if m.ID == self {
				log.Infoln("I am the leader")
			}
		}
	}

	log.Infoln("Stopping etcd")
	err = StopContainer.Start(containerName)
	require.NoError(t, err)
}
