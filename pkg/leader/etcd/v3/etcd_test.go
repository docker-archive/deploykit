package etcd

import (
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
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
			log.Info("etcd running")
			break
		}
	}

	defer etcd.StopContainer.Start(containerName)

	t.Run("AmILeader", testAmILeader)

	t.Run("StoreTest", testStore)

}

func testStore(t *testing.T) {

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

	store := Store{client}

	loc := "tcp://10.10.1.100:24864"
	u, err := url.Parse(loc)
	require.NoError(t, err)

	err = store.UpdateLocation(u)
	require.NoError(t, err)

	uu, err := store.GetLocation()
	require.NoError(t, err)

	require.Equal(t, u, uu)
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
