package mux

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	rpc_metadata "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"

	. "github.com/docker/infrakit/pkg/testing"
)

func TestMuxServer(t *testing.T) {

	pluginName := "metadata"
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
	require.Equal(t, []string{"region"},
		first(must(rpc_metadata.NewClient(nameFromPath(socketPath)+"/aws", socketPath)).Keys(types.PathFromString("."))))
	require.Equal(t, []string(nil),
		first(must(rpc_metadata.NewClient(nameFromPath(socketPath), socketPath)).Keys(types.PathFromString("aws"))))

	infoClient, err := client.NewPluginInfoClient(socketPath)
	require.NoError(t, err)
	info, err := infoClient.GetInfo()
	require.NoError(t, err)
	T(100).Infoln("info=", info)

	T(100).Infoln("Starting mux server")
	server, err = NewServer(":9090", "127.0.0.1:9090", func() discovery.Plugins {
		return lookup
	}, Options{})
	require.NoError(t, err)

	defer server.Stop()

	get := "http://localhost:9090/" + pluginName + rpc.URLAPI

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
