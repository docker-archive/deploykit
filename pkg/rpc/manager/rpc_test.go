package manager

import (
	"errors"
	"io/ioutil"
	"path"
	"testing"

	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	// IsLeader returns true if manager is leader
	DoIsLeader func() (bool, error)
}

func (t *testPlugin) IsLeader() (bool, error) {
	return t.DoIsLeader()
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "manager-impl-test")
}

func must(m manager.Manager, err error) manager.Manager {
	if err != nil {
		panic(err)
	}
	return m
}

func TestManagerIsLeader(t *testing.T) {
	socketPath := tempSocket()

	rawActual := make(chan bool, 1)
	expect := true

	server, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
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

	server, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
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
