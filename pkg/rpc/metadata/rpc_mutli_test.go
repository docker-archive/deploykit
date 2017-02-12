package metadata

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	plugin_metadata "github.com/docker/infrakit/pkg/plugin/metadata"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/metadata"
	testing_metadata "github.com/docker/infrakit/pkg/testing/metadata"
	_ "github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "metadata-impl-test")
}

func must(p metadata.Plugin, err error) metadata.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func first(a, b interface{}) interface{} {
	return a
}

func second(a, b interface{}) interface{} {
	return b
}

func TestMetadataMultiPluginList(t *testing.T) {
	socketPath := tempSocket()

	inputMetadataPathActual1 := make(chan []string, 1)

	inputMetadataPathActual2 := make(chan []string, 1)

	m := map[string]interface{}{}
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/metrics/instances/count"), 100, m)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path []string) ([]string, error) {
					inputMetadataPathActual1 <- path
					return plugin_metadata.List(path, m), nil
				},
			},
			"azure": &testing_metadata.Plugin{
				DoList: func(path []string) ([]string, error) {
					inputMetadataPathActual2 <- path
					return nil, errors.New("azure-error")
				},
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"region"}, first(must(NewClient(socketPath)).List(plugin_metadata.Path("aws"))))
	require.Error(t, second(must(NewClient(socketPath)).List(plugin_metadata.Path("azure"))).(error))

	require.Equal(t, []string{}, <-inputMetadataPathActual1)
	require.Equal(t, []string{}, <-inputMetadataPathActual2)

	server.Stop()
}

func TestMetadataMultiPluginList2(t *testing.T) {
	socketPath := tempSocket()

	m1 := map[string]interface{}{}
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m1)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m1)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m1)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m1)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m1)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/metrics/instances/count"), 100, m1)

	m2 := map[string]interface{}{}
	plugin_metadata.Put(plugin_metadata.Path("dc/us-1/vpc/vpc1/network/network1/id"), "id-network1", m2)
	plugin_metadata.Put(plugin_metadata.Path("dc/us-1/vpc/vpc2/network/network10/id"), "id-network10", m2)
	plugin_metadata.Put(plugin_metadata.Path("dc/us-1/vpc/vpc2/network/network11/id"), "id-network11", m2)
	plugin_metadata.Put(plugin_metadata.Path("dc/us-2/vpc/vpc21/network/network210/id"), "id-network210", m2)
	plugin_metadata.Put(plugin_metadata.Path("dc/us-2/vpc/vpc21/network/network211/id"), "id-network211", m2)
	plugin_metadata.Put(plugin_metadata.Path("dc/us-2/metrics/instances/count"), 100, m2)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path []string) ([]string, error) {
					res := plugin_metadata.List(path, m1)
					return res, nil
				},
			},
			"azure": &testing_metadata.Plugin{
				DoList: func(path []string) ([]string, error) {
					res := plugin_metadata.List(path, m2)
					return res, nil
				},
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"region"},
		first(must(NewClient(socketPath)).List(plugin_metadata.Path("aws"))))
	require.Equal(t, []string{"dc"},
		first(must(NewClient(socketPath)).List(plugin_metadata.Path("azure/"))))
	require.Equal(t, []string(nil),
		first(must(NewClient(socketPath)).List(plugin_metadata.Path("gce/"))))
	require.Equal(t, []string{"network10", "network11"},
		first(must(NewClient(socketPath)).List(plugin_metadata.Path("aws/region/us-west-1/vpc/vpc2/network"))))
	server.Stop()
}
