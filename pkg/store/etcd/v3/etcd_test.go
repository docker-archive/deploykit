package etcd

import (
	"path"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	"github.com/stretchr/testify/require"
)

const defaultKey = "groups.json"

func TestWithRealEtcd(t *testing.T) {

	if testutil.SkipTests("etcd") {
		t.SkipNow()
	}

	ip := etcd.LocalIP()
	containerName := "test-etcd-store"

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

	t.Run("SaveLoad", testSaveLoad)
}

func testSaveLoad(t *testing.T) {

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

	etcdClient, err := etcd.NewClient(options)
	require.NoError(t, err)
	snap, err := NewSnapshot(etcdClient, defaultKey)
	require.NoError(t, err)

	defer snap.Close()

	config := map[string]interface{}{
		"Group": map[string]interface{}{
			"managers": map[string]interface{}{
				"Instance":   "foo",
				"Flavor":     "bar",
				"Allocation": []interface{}{"a", "b", "c"},
			},
			"workers": map[string]interface{}{
				"Instance": "bar",
				"Flavor":   "baz",
			},
		},
	}

	err = snap.Save(config)
	require.NoError(t, err)

	config2 := map[string]interface{}{}
	require.NotEqual(t, config, config2)

	err = snap.Load(&config2)
	log.Info("snapshot from etcd", "config", config2)
	require.Equal(t, config, config2)

	// verify with the etcdctl client
	output, err := etcd.Get.Output(ip, path.Join(namespace, defaultKey))
	require.NoError(t, err)

	log.Info("read by etcdctl", "result", string(output))

	any2 := types.AnyString(strings.Trim(string(output), " \t\n"))
	config2 = map[string]interface{}{}

	err = any2.Decode(&config2)
	require.NoError(t, err)

	require.Equal(t, config, config2)

}
