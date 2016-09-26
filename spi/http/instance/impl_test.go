package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/docker/libmachete/plugin/util"
	"github.com/docker/libmachete/spi/instance"
	"github.com/stretchr/testify/require"
)

func listenAddr() string {
	return fmt.Sprintf("tcp://:%d", rand.Int()%10000+1000)
}

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

func TestInstancePluginValidate(t *testing.T) {

	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"name":"instance","type":"xlarge"}`))

	rawActual := make(chan json.RawMessage, 1)

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoValidate: func(req json.RawMessage) error {

			rawActual <- req

			return nil
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	instancePluginClient := PluginClient(callable)

	// Make call
	err = instancePluginClient.Validate(raw)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, raw, <-rawActual)
}

func TestInstancePluginValidateError(t *testing.T) {

	listen := listenAddr()
	raw := json.RawMessage([]byte(`{"name":"instance","type":"xlarge"}`))
	rawActual := make(chan json.RawMessage, 1)

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoValidate: func(req json.RawMessage) error {

			rawActual <- req

			return errors.New("whoops")
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	instancePluginClient := PluginClient(callable)

	// Make call
	err = instancePluginClient.Validate(raw)
	require.Error(t, err)
	require.Equal(t, "whoops", err.Error())

	close(stop)
	require.Equal(t, raw, <-rawActual)
}

func TestInstancePluginProvisionNil(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			return nil, nil
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	instancePluginClient := PluginClient(callable)

	// Make call
	var id *instance.ID
	id, err = instancePluginClient.Provision(spec)
	require.NoError(t, err)
	require.Nil(t, id)

	close(stop)

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginProvision(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			v := instance.ID("test")
			return &v, nil
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	instancePluginClient := PluginClient(callable)

	// Make call
	var id *instance.ID
	id, err = instancePluginClient.Provision(spec)
	require.NoError(t, err)
	require.Equal(t, "test", string(*id))

	close(stop)

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginProvisionError(t *testing.T) {
	listen := listenAddr()

	raw := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual := make(chan instance.Spec, 1)
	spec := instance.Spec{
		Properties: &raw,
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoProvision: func(req instance.Spec) (*instance.ID, error) {
			specActual <- req
			return nil, errors.New("nope")
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	instancePluginClient := PluginClient(callable)

	// Make call
	_, err = instancePluginClient.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "nope", err.Error())

	close(stop)

	require.Equal(t, spec, <-specActual)
}

func TestInstancePluginDestroy(t *testing.T) {
	listen := listenAddr()

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDestroy: func(req instance.ID) error {
			instActual <- req
			return nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	instancePluginClient := PluginClient(callable)

	// Make call
	err = instancePluginClient.Destroy(inst)
	require.NoError(t, err)

	close(stop)

	require.Equal(t, inst, <-instActual)
}

func TestInstancePluginDestroyError(t *testing.T) {
	listen := listenAddr()

	inst := instance.ID("hello")
	instActual := make(chan instance.ID, 1)

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDestroy: func(req instance.ID) error {
			instActual <- req
			return errors.New("can't do")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	instancePluginClient := PluginClient(callable)

	// Make call
	err = instancePluginClient.Destroy(inst)
	require.Error(t, err)
	require.Equal(t, "can't do", err.Error())

	close(stop)
	require.Equal(t, inst, <-instActual)
}

func TestInstancePluginDescribeInstancesNiInput(t *testing.T) {
	listen := listenAddr()

	var tags map[string]string
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	instancePluginClient := PluginClient(callable)

	// Make call
	l, err := instancePluginClient.DescribeInstances(tags)
	require.NoError(t, err)
	require.Equal(t, list, l)

	close(stop)
	require.Equal(t, tags, <-tagsActual)
}

func TestInstancePluginDescribeInstances(t *testing.T) {
	listen := listenAddr()

	tags := map[string]string{
		"foo": "bar",
	}
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, nil
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	instancePluginClient := PluginClient(callable)

	// Make call
	l, err := instancePluginClient.DescribeInstances(tags)
	require.NoError(t, err)
	require.Equal(t, list, l)

	close(stop)
	require.Equal(t, tags, <-tagsActual)
}

func TestInstancePluginDescribeInstancesError(t *testing.T) {
	listen := listenAddr()

	tags := map[string]string{
		"foo": "bar",
	}
	tagsActual := make(chan map[string]string, 1)
	list := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop")},
	}
	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
			tagsActual <- req
			return list, errors.New("bad")
		},
	}))
	require.NoError(t, err)
	callable, err := util.NewClient(listen)
	require.NoError(t, err)
	instancePluginClient := PluginClient(callable)

	// Make call
	_, err = instancePluginClient.DescribeInstances(tags)
	require.Error(t, err)
	require.Equal(t, "bad", err.Error())

	close(stop)
	require.Equal(t, tags, <-tagsActual)
}
