package etcd

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestWithRealEtcd(t *testing.T) {

	if testutil.SkipTests("etcd") {
		t.SkipNow()
	}

	ip := etcd.LocalIP()
	containerName := "test-etcd-leader"

	err := etcd.RunContainer.Start(ip, containerName)
	require.NoError(t, err)

	// wait until ready
	for {
		<-time.After(1 * time.Second)
		_, err := etcd.LsMembers.Output(ip)
		if err == nil {
			log.Infoln("etcd running")
			break
		}
	}

	defer etcd.StopContainer.Run(containerName)

	t.Run("AmILeader", testAmILeader)
}

func testAmILeader(t *testing.T) {

	if testutil.SkipTests("etcd") {
		t.SkipNow()
	}

	ip := etcd.LocalIP()
	options := etcd.Options{
		Config: clientv3.Config{
			Endpoints: []string{ip + ":2379"},
		},
		RequestTimeout: 1 * time.Second,
	}

	client, err := etcd.NewClient(options)
	require.NoError(t, err)

	defer client.Close()

	leader, err := AmILeader(context.Background(), client)
	require.NoError(t, err)
	require.True(t, leader)
}
