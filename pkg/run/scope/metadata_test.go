package scope

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	plugin_metadata "github.com/docker/infrakit/pkg/plugin/metadata"
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

func runUpdatableServer(t *testing.T, n string, m *map[string]interface{}) (rpc_server.Stoppable, string) {
	socketPath := tempSocket(n)
	server, err := rpc_server.StartPluginAtPath(socketPath, rpc_metadata.UpdatableServerWithNames(
		func() (map[string]metadata.Plugin, error) {
			return map[string]metadata.Plugin{
				".": plugin_metadata.NewUpdatablePlugin(
					&testing_metadata.Plugin{
						DoKeys: func(path types.Path) ([]string, error) {
							return types.List(path, m), nil
						},
						DoGet: func(path types.Path) (*types.Any, error) {
							return types.GetValue(path, m)
						},
					},
					func(proposed *types.Any) error {
						return proposed.Decode(m)
					},
				),
			}, nil
		}))
	require.NoError(t, err)
	return server, socketPath
}

func TestMetadataFunc(t *testing.T) {
	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)

	server, socket := runUpdatableServer(t, "vars", &m)

	scope := DefaultScope(discoveryFromPath(socket))

	v, err := MetadataFunc(scope)("vars/region/count")
	require.NoError(t, err)
	require.Equal(t, float64(3), v)

	_, err = MetadataFunc(scope)("vars/region/count", 100)
	require.NoError(t, err)

	v, err = MetadataFunc(scope)("vars/region/count")
	require.NoError(t, err)
	require.Equal(t, float64(100), v)

	v, err = MetadataFunc(scope)("vars/region/count", "500ms", "2s")
	require.NoError(t, err)
	require.Equal(t, float64(100), v)

	now := time.Now()
	v, err = MetadataFunc(scope)("vars/region/count/no/exist", "500ms", "3s")
	require.Error(t, err)
	require.True(t, IsExpired(err))
	require.True(t, 2*time.Second < time.Now().Sub(now)) // at least 2s have elapsed

	v, err = MetadataFunc(scope)("missing/region/count/no/exist", "500ms", "2s")
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = MetadataFunc(scope)("missing")
	require.NoError(t, err)
	require.Nil(t, v)

	server.Stop()
}

func TestDoSet(t *testing.T) {
	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)

	server, socket := runUpdatableServer(t, "vars", &m)

	resolver := DefaultScope(discoveryFromPath(socket)).Metadata

	v, err := doBlockingGet(resolver, "vars/region/count", 0, 0)
	require.NoError(t, err)
	require.Equal(t, float64(3), v)

	_, err = doSet(resolver, "vars/region/count", 100)
	require.NoError(t, err)

	v, err = doBlockingGet(resolver, "vars/region/count", 0, 0)
	require.NoError(t, err)
	require.Equal(t, float64(100), v)

	server.Stop()
}

func TestDoSetReadonly(t *testing.T) {
	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)

	server, socket := runServer(t, "set-vars", m)

	resolver := DefaultScope(discoveryFromPath(socket)).Metadata

	_, err := doSet(resolver, "set-vars/aws/region/new/value", 100)
	require.Error(t, err)
	require.True(t, IsReadonly(err))

	server.Stop()
}

func TestParseDuration(t *testing.T) {

	var d time.Duration
	var err error

	d, err = duration(1 * time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 1*time.Millisecond, d)

	d, err = duration("1ms")
	require.NoError(t, err)
	require.Equal(t, 1*time.Millisecond, d)

	d, err = duration([]byte("1ms"))
	require.NoError(t, err)
	require.Equal(t, 1*time.Millisecond, d)

}

func TestDoBlockingGet(t *testing.T) {
	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)
	types.Put(types.PathFromString("region/count2"), func() interface{} { return 10. }, m)

	calls := 0
	types.Put(types.PathFromString("region/calls"), func() interface{} {
		if calls == 0 {
			calls++
			return nil
		}
		v := calls
		calls++
		return v
	}, m)

	server, socket := runServer(t, "test-vars", m)

	resolver := DefaultScope(discoveryFromPath(socket)).Metadata

	for l, check := range map[string]func(*testing.T, interface{}, error){
		"test-vars/aws/region/count": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, float64(3), v) // TODO - we lost the int type...
		},
		"test-vars/aws/region/count2": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, float64(10), v) // TODO - we lost the int type...
		},
		"test-vars/aws/region/calls": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, float64(1), v) // TODO - we lost the int type...
		},
		"test-vars/not/found/region/count": func(t *testing.T, v interface{}, err error) {
			require.Error(t, err) // timeout
			require.True(t, IsExpired(err))
		},
		"test-vars/aws": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, true, v) // Test for existence
		},
		"test-vars": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, true, v) // Test for existence
		},
		"missing-vars": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Nil(t, v)
		},
		"missing-vars/region/count": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Nil(t, v)
		},
	} {
		c, err := doBlockingGet(resolver, l, 500*time.Millisecond, 1*time.Second)
		check(t, c, err)
	}

	calls = 0 // expect to return nil when calls == 0
	for l, check := range map[string]func(*testing.T, interface{}, error){
		"test-vars/aws/region/count": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, float64(3), v) // TODO - we lost the int type...
		},
		"test-vars/aws/region/count2": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, float64(10), v) // TODO - we lost the int type...
		},
		"test-vars/aws/region/calls": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Nil(t, v) // Returns nil because we have 0 for retries
		},
		"test-vars/not/found/region/count": func(t *testing.T, v interface{}, err error) {
			require.Nil(t, v)
			require.Error(t, err) // Error because test-vars exists but path is not found
		},
		"test-vars/aws": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, true, v) // Test for existence
		},
		"test-vars": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Equal(t, true, v) // Test for existence
		},
		"missing-vars": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Nil(t, v) // Test for existence
		},
		"missing-vars/region/count": func(t *testing.T, v interface{}, err error) {
			require.NoError(t, err)
			require.Nil(t, v) // The object is missing
		},
	} {
		c, err := doBlockingGet(resolver, l, 0, 0)
		check(t, c, err)
	}
	server.Stop()
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
