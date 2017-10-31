package scope

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	rpc_metadata "github.com/docker/infrakit/pkg/rpc/metadata"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/metadata"
	testing_metadata "github.com/docker/infrakit/pkg/testing/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func nameFromPath(p string, pp ...string) plugin.Name {
	if len(pp) == 0 {
		return plugin.Name(filepath.Base(p))
	}
	return plugin.Name(filepath.Base(p) + "/" + pp[0])
}

func tempSocket(n string) string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, n)
}

func discoveryFromPath(p string) func() discovery.Plugins {
	d, err := local.NewPluginDiscoveryWithDir(filepath.Dir(p))
	if err != nil {
		panic(err)
	}
	return func() discovery.Plugins {
		return d
	}
}

func runServer(t *testing.T, n string, m map[string]interface{}) (rpc_server.Stoppable, string) {
	socketPath := tempSocket(n)
	server, err := rpc_server.StartPluginAtPath(socketPath, rpc_metadata.ServerWithNames(
		func() (map[string]metadata.Plugin, error) {
			return map[string]metadata.Plugin{
				"aws": &testing_metadata.Plugin{
					DoKeys: func(path types.Path) ([]string, error) {
						return types.List(path, m), nil
					},
					DoGet: func(path types.Path) (*types.Any, error) {
						return types.GetValue(path, m)
					},
				},
				"azure": &testing_metadata.Plugin{
					DoKeys: func(path types.Path) ([]string, error) {
						return nil, errors.New("azure-error")
					},
					DoGet: func(path types.Path) (*types.Any, error) {
						return nil, errors.New("azure-error2")
					},
				},
			}, nil
		}))
	require.NoError(t, err)
	return server, socketPath
}

func TestMetadataResolver(t *testing.T) {
	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m)
	types.Put(types.PathFromString("region/us-west-2/metrics/instances/count"), 100, m)

	server, socket := runServer(t, "test-vars", m)

	for l, check := range map[string]func(*testing.T, *MetadataCall, error){
		"test-vars": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.NullPath, c.Key)
		},
		"test-vars/aws": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.NullPath, c.Key)
		},
		"test-vars/aws/region/count": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.PathFromString("region/count"), c.Key)
		},
		"test-vars/azure": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.NullPath, c.Key)
		},
		"test-vars/none": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.PathFromString("none"), c.Key) // Don't throw error because maybe later this can be resolved
		},
		"test-vars/none/sub": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, types.PathFromString("none/sub"), c.Key)
		},
		"test": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.Nil(t, c)
		},
		"test/aws": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.Nil(t, c)
		},
		"test/foo": func(t *testing.T, c *MetadataCall, err error) {
			require.NoError(t, err)
			require.Nil(t, c)
		},
	} {
		c, err := metadataPlugin(discoveryFromPath(socket), types.PathFromString(l))
		check(t, c, err)
	}

	server.Stop()
}
