package flavor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	testing_flavor "github.com/docker/infrakit/pkg/testing/flavor"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
)

var allocation = group.AllocationMethod{}
var index = group.Index{Group: group.ID("group"), Sequence: 0}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "flavor-impl-test")
}

func TestFlavorPluginValidate(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual := make(chan *types.Any, 1)
	inputFlavorProperties := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoValidate: func(flavorProperties *types.Any, allocation group.AllocationMethod) error {
			inputFlavorPropertiesActual <- flavorProperties
			return nil
		},
	}))
	require.NoError(t, err)

	require.NoError(t, must(NewClient(plugin.Name(name), socketPath)).Validate(inputFlavorProperties, allocation))

	server.Stop()

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
}

func TestFlavorPluginValidateError(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual := make(chan *types.Any, 1)
	inputFlavorProperties := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoValidate: func(flavorProperties *types.Any, allocation group.AllocationMethod) error {
			inputFlavorPropertiesActual <- flavorProperties
			return errors.New("something-went-wrong")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name), socketPath)).Validate(inputFlavorProperties, allocation)
	require.Error(t, err)
	require.Equal(t, "something-went-wrong", err.Error())

	server.Stop()
	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
}

func TestFlavorPluginPrepare(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual := make(chan *types.Any, 1)
	inputFlavorProperties := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)
	inputInstanceSpecActual := make(chan instance.Spec, 1)
	inputInstanceSpec := instance.Spec{
		Properties: inputFlavorProperties,
		Tags:       map[string]string{"foo": "bar"},
	}
	inputInstanceIndexActual := make(chan group.Index, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoPrepare: func(
			flavorProperties *types.Any,
			instanceSpec instance.Spec,
			allocation group.AllocationMethod,
			idx group.Index) (instance.Spec, error) {

			inputFlavorPropertiesActual <- flavorProperties
			inputInstanceSpecActual <- instanceSpec
			inputInstanceIndexActual <- idx
			return instanceSpec, nil
		},
	}))
	require.NoError(t, err)

	spec, err := must(NewClient(plugin.Name(name), socketPath)).Prepare(
		inputFlavorProperties,
		inputInstanceSpec,
		allocation,
		index)
	require.NoError(t, err)
	require.Equal(t, inputInstanceSpec, spec)

	server.Stop()

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
	require.Equal(t, inputInstanceSpec, <-inputInstanceSpecActual)
	require.Equal(t, index, <-inputInstanceIndexActual)
}

func TestFlavorPluginPrepareError(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual := make(chan *types.Any, 1)
	inputFlavorProperties := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)
	inputInstanceSpecActual := make(chan instance.Spec, 1)
	inputInstanceSpec := instance.Spec{
		Properties: inputFlavorProperties,
		Tags:       map[string]string{"foo": "bar"},
	}
	inputInstanceIndexActual := make(chan group.Index, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoPrepare: func(
			flavorProperties *types.Any,
			instanceSpec instance.Spec,
			allocation group.AllocationMethod,
			idx group.Index) (instance.Spec, error) {

			inputFlavorPropertiesActual <- flavorProperties
			inputInstanceSpecActual <- instanceSpec
			inputInstanceIndexActual <- idx
			return instanceSpec, errors.New("bad-thing-happened")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name(name), socketPath)).Prepare(
		inputFlavorProperties,
		inputInstanceSpec,
		allocation,
		index)
	require.Error(t, err)
	require.Equal(t, "bad-thing-happened", err.Error())

	server.Stop()

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
	require.Equal(t, inputInstanceSpec, <-inputInstanceSpecActual)
	require.Equal(t, index, <-inputInstanceIndexActual)
}

func TestFlavorPluginHealthy(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual := make(chan *types.Any, 1)
	inputInstanceActual := make(chan instance.Description, 1)
	inputProperties := types.AnyString("{}")
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoHealthy: func(properties *types.Any, inst instance.Description) (flavor.Health, error) {
			inputPropertiesActual <- properties
			inputInstanceActual <- inst
			return flavor.Healthy, nil
		},
	}))
	require.NoError(t, err)

	health, err := must(NewClient(plugin.Name(name), socketPath)).Healthy(inputProperties, inputInstance)
	require.NoError(t, err)
	require.Equal(t, flavor.Healthy, health)

	require.Equal(t, inputProperties, <-inputPropertiesActual)
	require.Equal(t, inputInstance, <-inputInstanceActual)
	server.Stop()
}

func TestFlavorPluginHealthyError(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual := make(chan *types.Any, 1)
	inputInstanceActual := make(chan instance.Description, 1)
	inputProperties := types.AnyString("{}")
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoHealthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			inputPropertiesActual <- flavorProperties
			inputInstanceActual <- inst
			return flavor.Unknown, errors.New("oh-noes")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name(name), socketPath)).Healthy(inputProperties, inputInstance)
	require.Error(t, err)
	require.Equal(t, "oh-noes", err.Error())

	require.Equal(t, inputProperties, <-inputPropertiesActual)
	require.Equal(t, inputInstance, <-inputInstanceActual)
	server.Stop()
}

func TestFlavorPluginDrain(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual := make(chan *types.Any, 1)
	inputInstanceActual := make(chan instance.Description, 1)
	inputProperties := types.AnyString("{}")
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoDrain: func(properties *types.Any, inst instance.Description) error {
			inputPropertiesActual <- properties
			inputInstanceActual <- inst
			return nil
		},
	}))
	require.NoError(t, err)

	require.NoError(t, must(NewClient(plugin.Name(name), socketPath)).Drain(inputProperties, inputInstance))

	require.Equal(t, inputProperties, <-inputPropertiesActual)
	require.Equal(t, inputInstance, <-inputInstanceActual)
	server.Stop()
}

func TestFlavorPluginDrainError(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual := make(chan *types.Any, 1)
	inputInstanceActual := make(chan instance.Description, 1)
	inputProperties := types.AnyString("{}")
	inputInstance := instance.Description{
		ID:   instance.ID("foo"),
		Tags: map[string]string{"foo": "bar"},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_flavor.Plugin{
		DoDrain: func(flavorProperties *types.Any, inst instance.Description) error {
			inputPropertiesActual <- flavorProperties
			inputInstanceActual <- inst
			return errors.New("oh-noes")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name), socketPath)).Drain(inputProperties, inputInstance)
	require.Error(t, err)
	require.Equal(t, "oh-noes", err.Error())

	require.Equal(t, inputProperties, <-inputPropertiesActual)
	require.Equal(t, inputInstance, <-inputInstanceActual)
	server.Stop()
}
