package metadata

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/metadata"
	testing_metadata "github.com/docker/infrakit/pkg/testing/metadata"
	"github.com/docker/infrakit/pkg/types"
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

func firstAny(a, b interface{}) *types.Any {
	v := first(a, b)
	return v.(*types.Any)
}

func second(a, b interface{}) interface{} {
	return b
}

func TestMetadataMultiPlugin(t *testing.T) {
	socketPath := tempSocket()

	inputMetadataPathListActual1 := make(chan []string, 1)
	inputMetadataPathGetActual1 := make(chan []string, 1)

	inputMetadataPathListActual2 := make(chan []string, 1)
	inputMetadataPathGetActual2 := make(chan []string, 1)

	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m)
	types.Put(types.PathFromString("region/us-west-2/metrics/instances/count"), 100, m)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					inputMetadataPathListActual1 <- path
					return types.List(path, m), nil
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					inputMetadataPathGetActual1 <- path
					return types.GetValue(path, m)
				},
			},
			"azure": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					inputMetadataPathListActual2 <- path
					return nil, errors.New("azure-error")
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					inputMetadataPathGetActual2 <- path
					return nil, errors.New("azure-error2")
				},
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"region"}, first(must(NewClient(socketPath)).List(types.PathFromString("aws"))))
	require.Error(t, second(must(NewClient(socketPath)).List(types.PathFromString("azure"))).(error))

	require.Equal(t, []string{"aws", "azure"},
		first(must(NewClient(socketPath)).List(types.PathFromString("/"))))

	require.Equal(t, []string{}, <-inputMetadataPathListActual1)
	require.Equal(t, []string{}, <-inputMetadataPathListActual2)

	require.Equal(t, "3", firstAny(must(NewClient(socketPath)).Get(types.PathFromString("aws/region/count"))).String())
	require.Error(t, second(must(NewClient(socketPath)).Get(types.PathFromString("azure"))).(error))

	require.Equal(t, []string{"region", "count"}, <-inputMetadataPathGetActual1)
	require.Equal(t, []string{}, <-inputMetadataPathGetActual2)

	server.Stop()
}

func TestMetadataMultiPlugin2(t *testing.T) {
	socketPath := tempSocket()

	m1 := map[string]interface{}{}
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m1)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m1)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m1)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m1)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m1)
	types.Put(types.PathFromString("region/us-west-2/metrics/instances/count"), 100, m1)

	m2 := map[string]interface{}{}
	types.Put(types.PathFromString("dc/us-1/vpc/vpc1/network/network1/id"), "id-network1", m2)
	types.Put(types.PathFromString("dc/us-1/vpc/vpc2/network/network10/id"), "id-network10", m2)
	types.Put(types.PathFromString("dc/us-1/vpc/vpc2/network/network11/id"), "id-network11", m2)
	types.Put(types.PathFromString("dc/us-2/vpc/vpc21/network/network210/id"), "id-network210", m2)
	types.Put(types.PathFromString("dc/us-2/vpc/vpc21/network/network211/id"), "id-network211", m2)
	types.Put(types.PathFromString("dc/us-2/metrics/instances/count"), 100, m2)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					res := types.List(path, m1)
					return res, nil
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					return types.GetValue(path, m1)
				},
			},
			"azure": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					res := types.List(path, m2)
					return res, nil
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					return types.GetValue(path, m2)
				},
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"aws", "azure"},
		first(must(NewClient(socketPath)).List(types.PathFromString(""))))
	require.Equal(t, []string{"region"},
		first(must(NewClient(socketPath)).List(types.PathFromString("aws"))))
	require.Equal(t, []string{"dc"},
		first(must(NewClient(socketPath)).List(types.PathFromString("azure/"))))
	require.Equal(t, []string(nil),
		first(must(NewClient(socketPath)).List(types.PathFromString("gce/"))))
	require.Equal(t, []string{"network10", "network11"},
		first(must(NewClient(socketPath)).List(types.PathFromString("aws/region/us-west-1/vpc/vpc2/network"))))

	require.Equal(t, "100",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("aws/region/us-west-2/metrics/instances/count"))).String())
	require.Equal(t, "{\"network\":{\"network210\":{\"id\":\"id-network210\"},\"network211\":{\"id\":\"id-network211\"}}}",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("azure/dc/us-2/vpc/vpc21"))).String())
	require.Nil(t, firstAny(must(NewClient(socketPath)).Get(types.PathFromString("aws/none"))))

	server.Stop()
}

func TestMetadataMultiPlugin3(t *testing.T) {
	socketPath := tempSocket()

	m0 := map[string]interface{}{}
	types.Put(types.PathFromString("metrics/instances/count"), 100, m0)
	types.Put(types.PathFromString("metrics/networks/count"), 10, m0)
	types.Put(types.PathFromString("metrics/workers/count"), 1000, m0)
	types.Put(types.PathFromString("metrics/managers/count"), 7, m0)

	m1 := map[string]interface{}{}
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m1)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m1)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m1)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m1)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m1)
	types.Put(types.PathFromString("region/us-west-2/metrics/instances/count"), 100, m1)

	m2 := map[string]interface{}{}
	types.Put(types.PathFromString("dc/us-1/vpc/vpc1/network/network1/id"), "id-network1", m2)
	types.Put(types.PathFromString("dc/us-1/vpc/vpc2/network/network10/id"), "id-network10", m2)
	types.Put(types.PathFromString("dc/us-1/vpc/vpc2/network/network11/id"), "id-network11", m2)
	types.Put(types.PathFromString("dc/us-2/vpc/vpc21/network/network210/id"), "id-network210", m2)
	types.Put(types.PathFromString("dc/us-2/vpc/vpc21/network/network211/id"), "id-network211", m2)
	types.Put(types.PathFromString("dc/us-2/metrics/instances/count"), 100, m2)

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServer(&testing_metadata.Plugin{
			DoList: func(path types.Path) ([]string, error) {
				res := types.List(path, m0)
				return res, nil
			},
			DoGet: func(path types.Path) (*types.Any, error) {
				return types.GetValue(path, m0)
			},
		}).WithTypes(map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					res := types.List(path, m1)
					return res, nil
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					return types.GetValue(path, m1)
				},
			},
			"azure": &testing_metadata.Plugin{
				DoList: func(path types.Path) ([]string, error) {
					res := types.List(path, m2)
					return res, nil
				},
				DoGet: func(path types.Path) (*types.Any, error) {
					return types.GetValue(path, m2)
				},
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"aws", "azure", "metrics"},
		first(must(NewClient(socketPath)).List(types.Path([]string{}))))
	require.Equal(t, []string{"aws", "azure", "metrics"},
		first(must(NewClient(socketPath)).List(types.PathFromString("/"))))
	require.Equal(t, []string{"region"},
		first(must(NewClient(socketPath)).List(types.PathFromString("aws"))))
	require.Equal(t, []string{"dc"},
		first(must(NewClient(socketPath)).List(types.PathFromString("azure/"))))
	require.Equal(t, []string(nil),
		first(must(NewClient(socketPath)).List(types.PathFromString("gce/"))))
	require.Equal(t, []string{"network10", "network11"},
		first(must(NewClient(socketPath)).List(types.PathFromString("aws/region/us-west-1/vpc/vpc2/network"))))
	require.Equal(t, []string{"aws", "azure", "metrics"},
		first(must(NewClient(socketPath)).List(types.PathFromString("."))))

	require.Equal(t, "100",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("metrics/instances/count"))).String())
	require.Equal(t, "{\"network\":{\"network210\":{\"id\":\"id-network210\"},\"network211\":{\"id\":\"id-network211\"}}}",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("azure/dc/us-2/vpc/vpc21"))).String())
	require.Nil(t, firstAny(must(NewClient(socketPath)).Get(types.PathFromString("aws/none"))))

	server.Stop()
}

func TestMetadataMultiPlugin4(t *testing.T) {
	socketPath := tempSocket()

	m0 := map[string]interface{}{}
	types.Put(types.PathFromString("metrics/instances/count"), 100, m0)
	types.Put(types.PathFromString("metrics/networks/count"), 10, m0)
	types.Put(types.PathFromString("metrics/workers/count"), 1000, m0)
	types.Put(types.PathFromString("metrics/managers/count"), 7, m0)

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServer(&testing_metadata.Plugin{
			DoList: func(path types.Path) ([]string, error) {
				res := types.List(path, m0)
				return res, nil
			},
			DoGet: func(path types.Path) (*types.Any, error) {
				return types.GetValue(path, m0)
			},
		}))
	require.NoError(t, err)

	require.Equal(t, []string{"metrics"},
		first(must(NewClient(socketPath)).List(types.PathFromString(""))))
	require.Equal(t, []string{"instances", "managers", "networks", "workers"},
		first(must(NewClient(socketPath)).List(types.PathFromString("metrics/"))))

	require.Equal(t, "100",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("metrics/instances/count"))).String())
	require.Equal(t, "{\"instances\":{\"count\":100},\"managers\":{\"count\":7},\"networks\":{\"count\":10},\"workers\":{\"count\":1000}}",
		firstAny(must(NewClient(socketPath)).Get(types.PathFromString("metrics"))).String())
	require.Nil(t, firstAny(must(NewClient(socketPath)).Get(types.PathFromString("aws/none"))))
	server.Stop()
}
