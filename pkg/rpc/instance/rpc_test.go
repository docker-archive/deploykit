package instance

import (
	"errors"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	testing_instance "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "instance-impl-test")
}

func TestInstancePluginValidate(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	raw := types.AnyString(`{"name":"instance","type":"xlarge"}`)

	rawActual := make(chan *types.Any, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoValidate: func(req *types.Any) error {

			rawActual <- req

			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Validate(raw)
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, raw, <-rawActual)
}

func TestInstancePluginValidateError(t *testing.T) {

	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	raw := types.AnyString(`{"name":"instance","type":"xlarge"}`)
	rawActual := make(chan *types.Any, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoValidate: func(req *types.Any) error {

			rawActual <- req

			return errors.New("whoops")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Validate(raw)
	require.Error(t, err)
	require.Equal(t, "whoops", err.Error())

	server.Stop()
	require.Equal(t, raw, <-rawActual)
}

func TestInstancePluginProvisionNil(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	raw := types.AnyString(`{"test":"foo"}`)
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			return nil, nil
		},
	}))
	require.NoError(t, err)

	var id *instance.ID
	id, err = must(NewClient(name, socketPath)).Provision(spec)
	require.NoError(t, err)
	require.Nil(t, id)

	server.Stop()

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginProvision(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	raw := types.AnyString(`{"test":"foo"}`)
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			v := instance.ID("test")
			return &v, nil
		},
	}))
	require.NoError(t, err)

	var id *instance.ID
	id, err = must(NewClient(name, socketPath)).Provision(spec)
	require.NoError(t, err)
	require.Equal(t, "test", string(*id))

	server.Stop()

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginProvisionError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	raw := types.AnyString(`{"test":"foo"}`)
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			return nil, errors.New("nope")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(name, socketPath)).Provision(spec)
	require.Error(t, err)
	require.Equal(t, "nope", err.Error())

	server.Stop()

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginLabel(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	instActual := make(chan instance.ID, 1)
	labelActual := make(chan map[string]string, 1)
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoLabel: func(req instance.ID, labels map[string]string) error {
			instActual <- req
			labelActual <- labels
			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Label(inst, labels)
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, inst, <-instActual)
	require.Equal(t, labels, <-labelActual)
}

func TestInstancePluginLabelError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}

	instActual := make(chan instance.ID, 1)
	labelActual := make(chan map[string]string, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoLabel: func(req instance.ID, labels map[string]string) error {
			instActual <- req
			labelActual <- labels
			return errors.New("can't do")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Label(inst, labels)
	require.Error(t, err)
	require.Equal(t, "can't do", err.Error())

	server.Stop()
	require.Equal(t, inst, <-instActual)
	require.Equal(t, labels, <-labelActual)
}

func TestInstancePluginDestroy(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoDestroy: func(req instance.ID, ctx instance.Context) error {
			instActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Destroy(inst, instance.Termination)
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, inst, <-instActual)
}

func TestInstancePluginDestroyError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoDestroy: func(req instance.ID, ctx instance.Context) error {
			instActual <- req
			return errors.New("can't do")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Destroy(inst, instance.Termination)
	require.Error(t, err)
	require.Equal(t, "can't do", err.Error())

	server.Stop()
	require.Equal(t, inst, <-instActual)
}

func TestInstancePluginDescribeInstancesNiInput(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	var tags map[string]string
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))

	l, err := must(NewClient(name, socketPath)).DescribeInstances(tags, false)
	require.NoError(t, err)
	require.Equal(t, list, l)

	server.Stop()
	require.Equal(t, tags, <-tagsActual)
}

func TestInstancePluginDescribeInstances(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	tags := map[string]string{
		"foo": "bar",
	}
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))
	require.NoError(t, err)

	l, err := must(NewClient(name, socketPath)).DescribeInstances(tags, false)
	require.NoError(t, err)
	require.Equal(t, list, l)

	server.Stop()
	require.Equal(t, tags, <-tagsActual)
}

func TestInstancePluginDescribeInstancesError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	tags := map[string]string{
		"foo": "bar",
	}
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_instance.Plugin{
		DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
			tagsActual <- req
			return list, errors.New("bad")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(name, socketPath)).DescribeInstances(tags, false)
	require.Error(t, err)
	require.Equal(t, "bad", err.Error())

	server.Stop()
	require.Equal(t, tags, <-tagsActual)
}
