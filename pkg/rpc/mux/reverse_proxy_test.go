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
	plugin_metadata "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	rpc_metadata "github.com/docker/infrakit/pkg/rpc/metadata"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/metadata"
	testing_metadata "github.com/docker/infrakit/pkg/testing/metadata"
	"github.com/docker/infrakit/pkg/types"
	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
	"gopkg.in/tylerb/graceful.v1"
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

	log.Infof("Listening at: %s", listen)

	go func() {
		err := gracefulServer.Serve(listener)
		if err != nil {
			log.Warningln(err)
		}
	}()

	return gracefulServer, nil
}

func startPlugin(t *testing.T, name string) (string, rpc_server.Stoppable) {
	socketPath := tempSocket(name)

	m := map[string]interface{}{}
	plugin_metadata.Put(plugin_metadata.Path("region/count"), 3, m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m)
	plugin_metadata.Put(plugin_metadata.Path("region/us-west-2/metrics/instances/count"), 100, m)

	server, err := rpc_server.StartPluginAtPath(socketPath, rpc_metadata.PluginServerWithTypes(
		map[string]metadata.Plugin{
			"aws": &testing_metadata.Plugin{
				DoList: func(path metadata.Path) ([]string, error) {
					return plugin_metadata.List(path, m), nil
				},
				DoGet: func(path metadata.Path) (*types.Any, error) {
					return plugin_metadata.GetValue(path, m)
				},
			},
		}))
	require.NoError(t, err)
	log.Infoln("started plugin", server, "as", name, "at", socketPath)

	return socketPath, server
}

func TestMuxPlugins(t *testing.T) {

	pluginName := "metadata1"
	socketPath, server := startPlugin(t, pluginName)
	defer server.Stop()

	lookup, err := discovery.NewPluginDiscoveryWithDirectory(filepath.Dir(socketPath))
	require.NoError(t, err)

	log.Infoln("checking to see if discovery works")
	all, err := lookup.List()
	require.NoError(t, err)
	require.Equal(t, 1, len(all))
	require.Equal(t, pluginName, all[pluginName].Name)

	log.Infoln("Basic client")
	require.Equal(t, []string{"region"},
		first(must(rpc_metadata.NewClient(socketPath)).List(plugin_metadata.Path("aws"))))

	infoClient := client.NewPluginInfoClient(socketPath)
	info, err := infoClient.GetInfo()
	require.NoError(t, err)
	log.Infoln("info=", info)

	log.Infoln("Starting mux")
	rp := NewReverseProxy(func() discovery.Plugins {
		return lookup
	})
	require.NotNil(t, rp)

	proxy, err := startProxy(t, ":8080", rp)
	require.NoError(t, err)
	defer proxy.Stop(10 * time.Second)

	get := "http://localhost:8080/" + pluginName + rpc.URLAPI

	log.Infoln("Basic info client:", get)
	resp, err := http.Get(get)
	defer resp.Body.Close()
	log.Infoln("resp=", resp, "err=", err)

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	any := types.AnyBytes(body)
	m := map[string]interface{}{}
	err = any.Decode(&m)
	require.NoError(t, err)
	require.Equal(t, "Metadata", m["Implements"].([]interface{})[0].(map[string]interface{})["Name"])
	log.Infoln("body=", string(body))

}
