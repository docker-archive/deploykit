package resource

import (
	"errors"
	"testing"

	"io/ioutil"
	"path"

	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/resource"
	testing_resource "github.com/docker/infrakit/pkg/testing/resource"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func must(p resource.Plugin, err error) resource.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "resource-impl-test")
}

func TestResourcePluginCommit(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoCommit: func(req resource.Spec, pretend bool) (string, error) {
			resourceSpecActual <- req
			return "details", nil
		},
	}))

	details, err := must(NewClient(socketPath)).Commit(resourceSpec, false)
	require.NoError(t, err)
	require.Equal(t, "details", details)

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}

func TestResourcePluginCommitError(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoCommit: func(req resource.Spec, pretend bool) (string, error) {
			resourceSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(socketPath)).Commit(resourceSpec, false)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}

func TestResourcePluginDestroy(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoDestroy: func(req resource.Spec, pretend bool) (string, error) {
			resourceSpecActual <- req
			return "details", nil
		},
	}))

	details, err := must(NewClient(socketPath)).Destroy(resourceSpec, false)
	require.NoError(t, err)
	require.Equal(t, "details", details)

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}

func TestResourcePluginDestroyError(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoDestroy: func(req resource.Spec, pretend bool) (string, error) {
			resourceSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(socketPath)).Destroy(resourceSpec, false)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}

func TestResourcePluginDescribeResources(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}
	details := "details"

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoDescribeResources: func(req resource.Spec) (string, error) {
			resourceSpecActual <- req
			return details, nil
		},
	}))

	detailsActual, err := must(NewClient(socketPath)).DescribeResources(resourceSpec)
	require.NoError(t, err)
	require.Equal(t, details, detailsActual)

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}

func TestResourcePluginDescribeResourcesError(t *testing.T) {
	socketPath := tempSocket()

	resourceSpecActual := make(chan resource.Spec, 1)
	resourceSpec := resource.Spec{
		ID:         resource.ID("resource"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_resource.Plugin{
		DoDescribeResources: func(req resource.Spec) (string, error) {
			resourceSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(socketPath)).DescribeResources(resourceSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	server.Stop()

	require.Equal(t, resourceSpec, <-resourceSpecActual)
}
