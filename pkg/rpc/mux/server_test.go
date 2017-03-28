package mux

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	plugin_metadata "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/client"
	rpc_metadata "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/types"
	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
)

func TestMuxServer(t *testing.T) {

	pluginName := "metadata"
	socketPath, server := startPlugin(t, pluginName)
	defer server.Stop()

	lookup, err := local.NewPluginDiscoveryWithDirectory(filepath.Dir(socketPath))
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

	log.Infoln("Starting mux server")
	server, err = NewServer(":9090", func() discovery.Plugins {
		return lookup
	})
	require.NoError(t, err)

	defer server.Stop()

	get := "http://localhost:9090/" + pluginName + rpc.URLAPI

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
