package group

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/docker/infrakit/plugin/util"
	"github.com/docker/infrakit/spi/group"
	"github.com/docker/infrakit/spi/instance"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	DoWatchGroup     func(grp group.Spec) error
	DoUnwatchGroup   func(id group.ID) error
	DoInspectGroup   func(id group.ID) (group.Description, error)
	DoDescribeUpdate func(updated group.Spec) (string, error)
	DoUpdateGroup    func(updated group.Spec) error
	DoStopUpdate     func(id group.ID) error
	DoDestroyGroup   func(id group.ID) error
}

func (t *testPlugin) WatchGroup(grp group.Spec) error {
	return t.DoWatchGroup(grp)
}
func (t *testPlugin) UnwatchGroup(id group.ID) error {
	return t.DoUnwatchGroup(id)
}
func (t *testPlugin) InspectGroup(id group.ID) (group.Description, error) {
	return t.DoInspectGroup(id)
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

func listenAddr() string {
	return fmt.Sprintf("tcp://:%d", rand.Int()%10000+1000)
}

func TestGroupPluginWatchGroup(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoWatchGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.WatchGroup(groupSpec)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginWatchGroupError(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoWatchGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return errors.New("error")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.WatchGroup(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginDescribeUpdate(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDescribeUpdate: func(req group.Spec) (string, error) {
			groupSpecActual <- req
			return "hello", nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	desc, err := groupPluginClient.DescribeUpdate(groupSpec)
	require.NoError(t, err)
	require.Equal(t, "hello", desc)

	close(stop)
	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginDescribeUpdateError(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDescribeUpdate: func(req group.Spec) (string, error) {
			groupSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	_, err = groupPluginClient.DescribeUpdate(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUpdateGroup(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoUpdateGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.UpdateGroup(groupSpec)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUpdateGroupError(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"foo":"bar"}`))
	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: &raw,
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoUpdateGroup: func(req group.Spec) error {
			groupSpecActual <- req
			return errors.New("error")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.UpdateGroup(groupSpec)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	close(stop)

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginUnwatchGroup(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoUnwatchGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.UnwatchGroup(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginUnwatchGroupError(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoUnwatchGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.UnwatchGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginStopUpdate(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoStopUpdate: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.StopUpdate(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginStopUpdateError(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoStopUpdate: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.StopUpdate(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroup(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.DestroyGroup(id)
	require.NoError(t, err)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroupError(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	err = groupPluginClient.DestroyGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroup(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)

	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoInspectGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	res, err := groupPluginClient.InspectGroup(id)
	require.NoError(t, err)
	require.Equal(t, desc, res)

	close(stop)
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroupError(t *testing.T) {
	listen := listenAddr()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoInspectGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, errors.New("no")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	groupPluginClient := PluginClient(callable)

	// Make call
	_, err = groupPluginClient.InspectGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	close(stop)
	require.Equal(t, id, <-idActual)
}
