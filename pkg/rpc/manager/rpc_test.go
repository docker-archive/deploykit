package manager

import (
	"errors"
	"io/ioutil"
	"path"
	"testing"

	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/stack"
	testing_manager "github.com/docker/infrakit/pkg/testing/manager"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "manager-impl-test")
}

func must(m stack.Interface, err error) stack.Interface {
	if err != nil {
		panic(err)
	}
	return m
}

func TestManagerIsLeader(t *testing.T) {
	socketPath := tempSocket()

	rawActual := make(chan bool, 1)
	expect := true

	server, err := server.StartPluginAtPath(socketPath, PluginServer(&testing_manager.Plugin{
		DoIsLeader: func() (bool, error) {

			rawActual <- expect

			return expect, nil
		},
	}))
	require.NoError(t, err)

	actual, err := must(NewClient(socketPath)).IsLeader()
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, expect, <-rawActual)
	require.Equal(t, expect, actual)
}

func TestManagerIsLeaderError(t *testing.T) {
	socketPath := tempSocket()

	called := make(chan struct{})
	expect := errors.New("backend-error")

	server, err := server.StartPluginAtPath(socketPath, PluginServer(&testing_manager.Plugin{
		DoIsLeader: func() (bool, error) {

			close(called)

			return false, expect
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(socketPath)).IsLeader()
	require.Error(t, err)
	<-called

	server.Stop()

}

func TestManagerEnforce(t *testing.T) {
	socketPath := tempSocket()

	rawActual := make(chan []types.Spec, 1)
	expect := []types.Spec{
		{
			Kind: "group",
			Metadata: types.Metadata{
				Name: "workers",
			},
			Properties: types.AnyValueMust(map[string]interface{}{"a": 1, "b": 2}),
		},
		{
			Kind: "group",
			Metadata: types.Metadata{
				Name: "managers",
			},
			Properties: types.AnyValueMust(map[string]interface{}{"a": 11, "b": 22}),
		},
	}

	m := &testing_manager.Plugin{
		DoEnforce: func(specs []types.Spec) error {

			rawActual <- specs

			return nil
		},
	}
	server, err := server.StartPluginAtPath(socketPath, PluginServer(m))
	require.NoError(t, err)

	err = must(NewClient(socketPath)).Enforce(expect)
	require.NoError(t, err)
	require.EqualValues(t, types.AnyValueMust(expect), types.AnyValueMust(<-rawActual))

	expectErr := errors.New("boom")

	// test for error
	m.DoEnforce = func(specs []types.Spec) error {
		return expectErr
	}
	err = must(NewClient(socketPath)).Enforce(expect)
	require.Error(t, err)
	require.Equal(t, expectErr.Error(), err.Error())

	server.Stop()

}

func TestManagerTerminate(t *testing.T) {
	socketPath := tempSocket()

	rawActual := make(chan []types.Spec, 1)
	expect := []types.Spec{
		{
			Kind: "group",
			Metadata: types.Metadata{
				Name: "workers",
			},
			Properties: types.AnyValueMust(map[string]interface{}{"a": 1, "b": 2}),
		},
		{
			Kind: "group",
			Metadata: types.Metadata{
				Name: "managers",
			},
			Properties: types.AnyValueMust(map[string]interface{}{"a": 11, "b": 22}),
		},
	}

	m := &testing_manager.Plugin{
		DoTerminate: func(specs []types.Spec) error {

			rawActual <- specs

			return nil
		},
	}
	server, err := server.StartPluginAtPath(socketPath, PluginServer(m))
	require.NoError(t, err)

	err = must(NewClient(socketPath)).Terminate(expect)
	require.NoError(t, err)
	require.EqualValues(t, types.AnyValueMust(expect), types.AnyValueMust(<-rawActual))

	expectErr := errors.New("boom")

	// test for error
	m.DoTerminate = func(specs []types.Spec) error {
		return expectErr
	}
	err = must(NewClient(socketPath)).Terminate(expect)
	require.Error(t, err)
	require.Equal(t, expectErr.Error(), err.Error())

	server.Stop()

}

func TestManagerInspect(t *testing.T) {
	socketPath := tempSocket()

	expect := []types.Object{
		{
			Spec: types.Spec{
				Kind: "group",
				Metadata: types.Metadata{
					Name: "workers",
				},
				Properties: types.AnyValueMust(map[string]interface{}{"a": 1, "b": 2}),
			},
		},
		{
			Spec: types.Spec{
				Kind: "group",
				Metadata: types.Metadata{
					Name: "managers",
				},
				Properties: types.AnyValueMust(map[string]interface{}{"a": 11, "b": 22}),
			},
		},
	}

	m := &testing_manager.Plugin{
		DoInspect: func() ([]types.Object, error) {
			return expect, nil
		},
	}
	server, err := server.StartPluginAtPath(socketPath, PluginServer(m))
	require.NoError(t, err)

	objects, err := must(NewClient(socketPath)).Inspect()
	require.NoError(t, err)
	require.EqualValues(t, types.AnyValueMust(expect), types.AnyValueMust(objects))

	expectErr := errors.New("boom")

	// test for error
	m.DoInspect = func() ([]types.Object, error) {
		return nil, expectErr
	}
	_, err = must(NewClient(socketPath)).Inspect()
	require.Error(t, err)
	require.Equal(t, expectErr.Error(), err.Error())

	server.Stop()

}
