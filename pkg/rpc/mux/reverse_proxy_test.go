package mux

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	rpc_metadata "github.com/docker/infrakit/pkg/rpc/metadata"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/metadata"
	testing_metadata "github.com/docker/infrakit/pkg/testing/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/tylerb/graceful.v1"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestPluginNameFromURL(t *testing.T) {

	u, err := url.Parse("http://host:2302/foo/bar")
	require.NoError(t, err)
	require.Equal(t, "foo", pluginName(u))

	u, err = url.Parse("http://host:2302/foo")
	require.NoError(t, err)
	require.Equal(t, "foo", pluginName(u))

	u, err = url.Parse("http://host:2302")
	require.NoError(t, err)
	require.Equal(t, "", pluginName(u))

	u, err = url.Parse("http://host:2302//")
	require.NoError(t, err)
	require.Equal(t, "", pluginName(u))
}

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

func must(p metadata.Plugin, err error) metadata.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func first(a, b interface{}) interface{} {
	return a
}

func startProxy(t *testing.T, listen string, rp *ReverseProxy) (*graceful.Server, error) {
	gracefulServer := &graceful.Server{
		Timeout: 10 * time.Second,
		Server:  &http.Server{Addr: listen, Handler: rp},
	}

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	T(100).Infof("Listening at: %s", listen)

	go func() {
		TMustNoError(gracefulServer.Serve(listener))
	}()

	return gracefulServer, nil
}

func startPlugin(t *testing.T, name string) (string, rpc_server.Stoppable) {
	socketPath := tempSocket(name)

	m := map[string]interface{}{}
	types.Put(types.PathFromString("region/count"), 3, m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m)
	types.Put(types.PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m)
	types.Put(types.PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m)
	types.Put(types.PathFromString("region/us-west-2/metrics/instances/count"), 100, m)

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
			}, nil
		}))
	require.NoError(t, err)
	T(100).Infoln("started plugin", server, "as", name, "at", socketPath)

	return socketPath, server
}

func TestMuxPlugins(t *testing.T) {

	pluginName := "metadata1"
	socketPath, server := startPlugin(t, pluginName)
	defer server.Stop()

	lookup, err := local.NewPluginDiscoveryWithDir(filepath.Dir(socketPath))
	require.NoError(t, err)

	T(100).Infoln("checking to see if discovery works")
	all, err := lookup.List()
	require.NoError(t, err)
	require.Equal(t, 1, len(all))
	require.Equal(t, pluginName, all[pluginName].Name)

	T(100).Infoln("Basic client")
	require.Equal(t, []string(nil),
		first(must(rpc_metadata.NewClient(nameFromPath(socketPath), socketPath)).Keys(types.PathFromString("aws"))))
	require.Equal(t, []string{"region"},
		first(must(rpc_metadata.NewClient(nameFromPath(socketPath)+"/aws", socketPath)).Keys(types.PathFromString("."))))

	infoClient, err := client.NewPluginInfoClient(socketPath)
	require.NoError(t, err)
	info, err := infoClient.GetInfo()
	require.NoError(t, err)
	T(100).Infoln("info=", info)

	T(100).Infoln("Starting mux")
	rp := NewReverseProxy(func() discovery.Plugins {
		return lookup
	})
	require.NotNil(t, rp)

	proxy, err := startProxy(t, ":24863", rp)
	require.NoError(t, err)
	defer proxy.Stop(10 * time.Second)

	get := "http://localhost:24863/" + pluginName + rpc.URLAPI

	T(100).Infoln("Basic info client:", get)
	resp, err := http.Get(get)
	require.NoError(t, err)
	defer resp.Body.Close()
	T(100).Infoln("resp=", resp, "err=", err)

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	any := types.AnyBytes(body)
	m := map[string]interface{}{}
	err = any.Decode(&m)
	require.NoError(t, err)
	require.Equal(t, "Metadata", m["Implements"].([]interface{})[0].(map[string]interface{})["Name"])
	T(100).Infoln("body=", string(body))

}
