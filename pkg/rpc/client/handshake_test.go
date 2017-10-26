package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/stretchr/testify/require"
)

var apiSpec = spi.InterfaceSpec{
	Name:    "TestPlugin",
	Version: "0.1.0",
}

func startPluginServer(t *testing.T) (server.Stoppable, string) {
	dir, err := ioutil.TempDir("", "infrakit_handshake_test")
	require.NoError(t, err)

	name := "instance"
	socket := filepath.Join(dir, name)

	testServer, err := server.StartPluginAtPath(socket, &TestPlugin{spec: apiSpec})
	require.NoError(t, err)
	return testServer, socket
}

func TestErrVersionMismatch(t *testing.T) {
	var e error

	e = errVersionMismatch("test")
	require.True(t, IsErrVersionMismatch(e))

	e = fmt.Errorf("untyped")
	require.False(t, IsErrVersionMismatch(e))
}

func TestHandshakeSuccess(t *testing.T) {
	testServer, socket := startPluginServer(t)
	defer testServer.Stop()

	r, err := New(socket, apiSpec)
	require.NoError(t, err)
	client := rpcClient{client: r}
	require.NoError(t, client.DoSomething())
}

func TestHandshakeFailVersion(t *testing.T) {
	testServer, socket := startPluginServer(t)
	defer testServer.Stop()

	r, err := New(socket, spi.InterfaceSpec{Name: "TestPlugin", Version: "0.2.0"})
	require.Error(t, err)

	client := rpcClient{client: r}
	err = client.DoSomething()
	require.Error(t, err)
	require.Equal(t, "Plugin supports TestPlugin interface version 0.1.0, client requires 0.2.0", err.Error())
}

func TestHandshakeFailWrongAPI(t *testing.T) {
	testServer, socket := startPluginServer(t)
	defer testServer.Stop()

	r, err := New(socket, spi.InterfaceSpec{Name: "OtherPlugin", Version: "0.1.0"})
	require.Error(t, err)

	client := rpcClient{client: r}
	err = client.DoSomething()
	require.Error(t, err)
	require.Equal(t, "Plugin does not support interface OtherPlugin/0.1.0", err.Error())
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
	spec spi.InterfaceSpec
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *TestPlugin) ImplementedInterface() spi.InterfaceSpec {
	return p.spec
}

// Types returns the types
func (p *TestPlugin) Types() []string {
	return []string{"."}
}

// EmptyMessage is an empty test message.
type EmptyMessage struct {
}

// DoSomething is an empty test RPC.
func (p *TestPlugin) DoSomething(_ *http.Request, req *EmptyMessage, resp *EmptyMessage) error {
	return nil
}
