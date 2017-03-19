package etcd

import (
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/etcd/v3"
	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
)

func TestWithRealEtcd(t *testing.T) {
	ip := etcd.LocalIP()
	containerName := "test-etcd-store"

	err := etcd.RunContainer.Start(ip, containerName)
	require.NoError(t, err)

	// wait until readyr
	for {
		<-time.After(1 * time.Second)
		_, err := etcd.LsMembers.Output(ip)
		if err == nil {
			log.Infoln("etcd running")
			break
		}
	}

	defer etcd.StopContainer.Run(containerName)

	t.Run("SaveLoad", testSaveLoad)
}

func testSaveLoad(t *testing.T) {
	ip := etcd.LocalIP()
	options := etcd.Options{
		Config: clientv3.Config{
			Endpoints: []string{ip + ":2379"},
		},
		RequestTimeout: 1 * time.Second,
	}

	snap, err := NewSnapshot(options)
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
	log.Infoln("snapshot from etcd:", config2)
	require.Equal(t, config, config2)

	// verify with the etcdctl client
	output, err := etcd.Get.Output(ip, DefaultKey)
	require.NoError(t, err)

	log.Infoln("read by etcdctl:", string(output))

	any2 := types.AnyString(strings.Trim(string(output), " \t\n"))
	config2 = map[string]interface{}{}

	err = any2.Decode(&config2)
	require.NoError(t, err)

	require.Equal(t, config, config2)

}
