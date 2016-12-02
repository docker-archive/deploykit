package client

import (
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
)

var apiSpec = spi.APISpec{
	Name:    "TestPlugin",
	Version: "0.1.0",
}

func TestHandshakeSuccess(t *testing.T) {
	dir, err := ioutil.TempDir("", "infrakit_handshake_test")
	require.NoError(t, err)

	name := "instance"
	socket := filepath.Join(dir, name)

	instanceServer, err := server.StartPluginAtPath(socket, &TestPlugin{spec: apiSpec})
	require.NoError(t, err)
	defer instanceServer.Stop()

	client := rpcClient{client: New(socket, apiSpec)}
	require.NoError(t, client.DoSomething())
}

func TestHandshakeFail(t *testing.T) {
	dir, err := ioutil.TempDir("", "infrakit_handshake_test")
	require.NoError(t, err)

	name := "instance"
	socket := filepath.Join(dir, name)

	instanceServer, err := server.StartPluginAtPath(socket, &TestPlugin{spec: apiSpec})
	require.NoError(t, err)
	defer instanceServer.Stop()

	client := rpcClient{client: New(socket, spi.APISpec{Name: "TestPlugin", Version: "0.2.0"})}
	err = client.DoSomething()
	require.Error(t, err)
	require.Equal(t, "Plugin supports TestPlugin API version 0.1.0, client requires 0.2.0", err.Error())
}

type rpcClient struct {
	client Client
}

func (c rpcClient) DoSomething() error {
	req := EmptyMessage{}
	resp := EmptyMessage{}
	return c.client.Call("TestPlugin.DoSomething", req, &resp)
}

// TestPlugin is an RPC service for this unit test.
type TestPlugin struct {
	spec spi.APISpec
}

// APISpec returns the API implemented by this RPC service.
func (p *TestPlugin) APISpec() spi.APISpec {
	return p.spec
}

// EmptyMessage is an empty test message.
type EmptyMessage struct {
}

// DoSomething is an empty test RPC.
func (p *TestPlugin) DoSomething(_ *http.Request, req *EmptyMessage, resp *EmptyMessage) error {
	return nil
}
