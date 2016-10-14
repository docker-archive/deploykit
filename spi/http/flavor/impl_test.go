package flavor

import (
	"encoding/json"
	"errors"
	"testing"

	plugin_client "github.com/docker/infrakit/plugin/util/client"
	"github.com/docker/infrakit/plugin/util/server"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/instance"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "flavor-impl-test")
}

type testPlugin struct {
	DoValidate func(flavorProperties json.RawMessage) (flavor.AllocationMethod, error)
	DoPrepare  func(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error)
	DoHealthy  func(inst instance.Description) (bool, error)
}

func (t *testPlugin) Validate(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
	return t.DoValidate(flavorProperties)
}
func (t *testPlugin) Prepare(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	return t.DoPrepare(flavorProperties, spec)
}
func (t *testPlugin) Healthy(inst instance.Description) (bool, error) {
	return t.DoHealthy(inst)
}

func TestFlavorPluginValidate(t *testing.T) {
	socketPath := tempSocket()

	inputFlavorPropertiesActual := make(chan json.RawMessage, 1)
	inputFlavorProperties := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))

	allocation := flavor.AllocationMethod{
		Size: 10,
	}
	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoValidate: func(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
			inputFlavorPropertiesActual <- flavorProperties
			return allocation, nil
		},
	}))
	require.NoError(t, err)

	alloc, err := PluginClient(plugin_client.New(socketPath)).Validate(inputFlavorProperties)
	require.NoError(t, err)
	require.Equal(t, allocation, alloc)

	close(stop)

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
}

func TestFlavorPluginValidateError(t *testing.T) {
	socketPath := tempSocket()

	inputFlavorPropertiesActual := make(chan json.RawMessage, 1)
	inputFlavorProperties := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))
	allocation := flavor.AllocationMethod{
		Size:       1,
		LogicalIDs: []instance.LogicalID{instance.LogicalID("overlord")},
	}

	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoValidate: func(flavorProperties json.RawMessage) (flavor.AllocationMethod, error) {
			inputFlavorPropertiesActual <- flavorProperties
			return allocation, errors.New("something-went-wrong")
		},
	}))
	require.NoError(t, err)

	_, err = PluginClient(plugin_client.New(socketPath)).Validate(inputFlavorProperties)
	require.Error(t, err)
	require.Equal(t, "something-went-wrong", err.Error())

	close(stop)
	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
}

func TestFlavorPluginPrepare(t *testing.T) {
	socketPath := tempSocket()

	inputFlavorPropertiesActual := make(chan json.RawMessage, 1)
	inputFlavorProperties := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))
	inputInstanceSpecActual := make(chan instance.Spec, 1)
	inputInstanceSpec := instance.Spec{
		Properties: &inputFlavorProperties,
		Tags:       map[string]string{"foo": "bar"},
	}

	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoPrepare: func(flavorProperties json.RawMessage, instanceSpec instance.Spec) (instance.Spec, error) {

			inputFlavorPropertiesActual <- flavorProperties
			inputInstanceSpecActual <- instanceSpec

			return instanceSpec, nil
		},
	}))
	require.NoError(t, err)

	spec, err := PluginClient(plugin_client.New(socketPath)).Prepare(inputFlavorProperties, inputInstanceSpec)
	require.NoError(t, err)
	require.Equal(t, inputInstanceSpec, spec)

	close(stop)

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
	require.Equal(t, inputInstanceSpec, <-inputInstanceSpecActual)
}

func TestFlavorPluginPrepareError(t *testing.T) {
	socketPath := tempSocket()

	inputFlavorPropertiesActual := make(chan json.RawMessage, 1)
	inputFlavorProperties := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))
	inputInstanceSpecActual := make(chan instance.Spec, 1)
	inputInstanceSpec := instance.Spec{
		Properties: &inputFlavorProperties,
		Tags:       map[string]string{"foo": "bar"},
	}

	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoPrepare: func(flavorProperties json.RawMessage, instanceSpec instance.Spec) (instance.Spec, error) {

			inputFlavorPropertiesActual <- flavorProperties
			inputInstanceSpecActual <- instanceSpec

			return instanceSpec, errors.New("bad-thing-happened")
		},
	}))
	require.NoError(t, err)

	_, err = PluginClient(plugin_client.New(socketPath)).Prepare(inputFlavorProperties, inputInstanceSpec)
	require.Error(t, err)
	require.Equal(t, "bad-thing-happened", err.Error())

	close(stop)

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
	require.Equal(t, inputInstanceSpec, <-inputInstanceSpecActual)
}

func TestFlavorPluginHealthy(t *testing.T) {
	socketPath := tempSocket()

	inputInstanceActual := make(chan instance.Description, 1)
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoHealthy: func(inst instance.Description) (bool, error) {
			inputInstanceActual <- inst
			return true, nil
		},
	}))
	require.NoError(t, err)

	healthy, err := PluginClient(plugin_client.New(socketPath)).Healthy(inputInstance)
	require.NoError(t, err)
	require.True(t, healthy)

	require.Equal(t, inputInstance, <-inputInstanceActual)
	close(stop)
}

func TestFlavorPluginHealthyError(t *testing.T) {
	socketPath := tempSocket()

	inputInstanceActual := make(chan instance.Description, 1)
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	stop, _, err := server.StartPluginAtPath(socketPath, PluginServer(&testPlugin{
		DoHealthy: func(inst instance.Description) (bool, error) {
			inputInstanceActual <- inst
			return true, errors.New("oh-noes")
		},
	}))
	require.NoError(t, err)

	_, err = PluginClient(plugin_client.New(socketPath)).Healthy(inputInstance)
	require.Error(t, err)
	require.Equal(t, "oh-noes", err.Error())

	require.Equal(t, inputInstance, <-inputInstanceActual)
	close(stop)
}
