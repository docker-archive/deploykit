package group

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/docker/infrakit/rpc"
	"github.com/docker/infrakit/spi/group"
	"github.com/docker/infrakit/spi/instance"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
)

type testPlugin struct {
	DoCommitGroup   func(grp group.Spec, pretend bool) (string, error)
	DoFreeGroup     func(id group.ID) error
	DoDescribeGroup func(id group.ID) (group.Description, error)
	DoDestroyGroup  func(id group.ID) error
	DoInspectGroups func() ([]group.Spec, error)
}

func testClient(t *testing.T, socket string) group.Plugin {
	cl, err := NewClient("unix", socket)
	require.NoError(t, err)
	return cl
}

func (t *testPlugin) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	return t.DoCommitGroup(grp, pretend)
}
func (t *testPlugin) FreeGroup(id group.ID) error {
	return t.DoFreeGroup(id)
}
func (t *testPlugin) DescribeGroup(id group.ID) (group.Description, error) {
	return t.DoDescribeGroup(id)
}
func (t *testPlugin) DestroyGroup(id group.ID) error {
	return t.DoDestroyGroup(id)
}
func (t *testPlugin) InspectGroups() ([]group.Spec, error) {
	return t.DoInspectGroups()
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "group-impl-test")
}

func TestGroupPluginCommitGroup(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
			groupSpecActual <- req
			return "commit details", nil
		},
	}))
	require.NoError(t, err)

	// Make call
	details, err := testClient(t, socketPath).CommitGroup(groupSpec, false)
	require.NoError(t, err)
	require.Equal(t, "commit details", details)

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginCommitGroupError(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
			groupSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = testClient(t, socketPath).CommitGroup(groupSpec, false)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginFreeGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoFreeGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).FreeGroup(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginFreeGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoFreeGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).FreeGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).DestroyGroup(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).DestroyGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)

	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, nil
		},
	}))
	require.NoError(t, err)

	res, err := testClient(t, socketPath).DescribeGroup(id)
	require.NoError(t, err)
	require.Equal(t, desc, res)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, errors.New("no")
		},
	}))
	require.NoError(t, err)

	_, err = testClient(t, socketPath).DescribeGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}
