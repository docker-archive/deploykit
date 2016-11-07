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
	DoWatchGroup     func(grp group.Spec) error
	DoUnwatchGroup   func(id group.ID) error
	DoDescribeGroup  func(id group.ID) (group.Description, error)
	DoDescribeUpdate func(updated group.Spec) (string, error)
	DoUpdateGroup    func(updated group.Spec) error
	DoStopUpdate     func(id group.ID) error
	DoDestroyGroup   func(id group.ID) error
	DoInspectGroups  func() ([]group.Spec, error)
}

func testClient(t *testing.T, socket string) group.Plugin {
	cl, err := NewClient("unix", socket)
	require.NoError(t, err)
	return cl
}

func (t *testPlugin) WatchGroup(grp group.Spec) error {
	return t.DoWatchGroup(grp)
}
func (t *testPlugin) UnwatchGroup(id group.ID) error {
	return t.DoUnwatchGroup(id)
}
func (t *testPlugin) DescribeGroup(id group.ID) (group.Description, error) {
	return t.DoDescribeGroup(id)
}
func (t *testPlugin) DescribeUpdate(updated group.Spec) (string, error) {
	return t.DoDescribeUpdate(updated)
}
func (t *testPlugin) UpdateGroup(updated group.Spec) error {
	return t.DoUpdateGroup(updated)
}
func (t *testPlugin) StopUpdate(id group.ID) error {
	return t.DoStopUpdate(id)
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

func TestGroupPluginWatchGroup(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoWatchGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	// Make call
	err = testClient(t, socketPath).WatchGroup(groupSpec)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginWatchGroupError(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoWatchGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return errors.New("error")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).WatchGroup(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginDescribeUpdate(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeUpdate: func(req group.Spec) (string, error) {
			groupSpecActual <- req
			return "hello", nil
		},
	}))
	require.NoError(t, err)

	desc, err := testClient(t, socketPath).DescribeUpdate(groupSpec)
	require.NoError(t, err)
	require.Equal(t, "hello", desc)

	close(stop)
	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginDescribeUpdateError(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeUpdate: func(req group.Spec) (string, error) {
			groupSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = testClient(t, socketPath).DescribeUpdate(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUpdateGroup(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoUpdateGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).UpdateGroup(groupSpec)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUpdateGroupError(t *testing.T) {
	socketPath := tempSocket()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoUpdateGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return errors.New("error")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).UpdateGroup(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUnwatchGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoUnwatchGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).UnwatchGroup(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginUnwatchGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoUnwatchGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).UnwatchGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginStopUpdate(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoStopUpdate: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).StopUpdate(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginStopUpdateError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := rpc.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoStopUpdate: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = testClient(t, socketPath).StopUpdate(id)
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
