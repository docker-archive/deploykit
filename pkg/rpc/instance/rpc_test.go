package instance

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	// Validate performs local validation on a provision request.
	DoValidate func(req json.RawMessage) error

	// Provision creates a new instance based on the spec.
	DoProvision func(spec instance.Spec) (*instance.ID, error)

	// Destroy terminates an existing instance.
	DoDestroy func(instance instance.ID) error

	// DescribeInstances returns descriptions of all instances matching all of the provided tags.
	DoDescribeInstances func(tags map[string]string) ([]instance.Description, error)
}

func (t *testPlugin) Validate(req json.RawMessage) error {
	return t.DoValidate(req)
}
func (t *testPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	return t.DoProvision(spec)
}
func (t *testPlugin) Destroy(instance instance.ID) error {
	return t.DoDestroy(instance)
}
func (t *testPlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return t.DoDescribeInstances(tags)
}

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

	raw := json.RawMessage([]byte(`{"name":"instance","type":"xlarge"}`))

	rawActual := make(chan json.RawMessage, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoValidate: func(req json.RawMessage) error {

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

	raw := json.RawMessage([]byte(`{"name":"instance","type":"xlarge"}`))
	rawActual := make(chan json.RawMessage, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoValidate: func(req json.RawMessage) error {

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

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
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

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
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

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
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

func TestInstancePluginDestroy(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDestroy: func(req instance.ID) error {
			instActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Destroy(inst)
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, inst, <-instActual)
}

func TestInstancePluginDestroyError(t *testing.T) {
	socketPath := tempSocket()
	name := plugin.Name(filepath.Base(socketPath))

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDestroy: func(req instance.ID) error {
			instActual <- req
			return errors.New("can't do")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(name, socketPath)).Destroy(inst)
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
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))

	l, err := must(NewClient(name, socketPath)).DescribeInstances(tags)
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
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))
	require.NoError(t, err)

	l, err := must(NewClient(name, socketPath)).DescribeInstances(tags)
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
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, errors.New("bad")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(name, socketPath)).DescribeInstances(tags)
	require.Error(t, err)
	require.Equal(t, "bad", err.Error())

	server.Stop()
	require.Equal(t, tags, <-tagsActual)
}
