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
)

func must(p flavor.Plugin, err error) flavor.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func TestFlavorMultiPluginValidate(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual1 := make(chan *types.Any, 1)
	inputFlavorProperties1 := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)

	inputFlavorPropertiesActual2 := make(chan *types.Any, 1)
	inputFlavorProperties2 := types.AnyString(`{"flavor":"zookeeper","role":"follower"}`)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testing_flavor.Plugin{
				DoValidate: func(flavorProperties *types.Any, allocation group.AllocationMethod) error {
					inputFlavorPropertiesActual1 <- flavorProperties
					return nil
				},
			},
			"type2": &testing_flavor.Plugin{
				DoValidate: func(flavorProperties *types.Any, allocation group.AllocationMethod) error {
					inputFlavorPropertiesActual2 <- flavorProperties
					return errors.New("something-went-wrong")
				},
			},
		}))
	require.NoError(t, err)

	require.NoError(t, must(NewClient(plugin.Name(name+"/type1"), socketPath)).Validate(inputFlavorProperties1, allocation))

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Validate(inputFlavorProperties2, allocation)
	require.Error(t, err)
	require.Equal(t, "something-went-wrong", err.Error())

	server.Stop()

	require.Equal(t, inputFlavorProperties1, <-inputFlavorPropertiesActual1)
	require.Equal(t, inputFlavorProperties2, <-inputFlavorPropertiesActual2)
}

func TestFlavorMultiPluginPrepare(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputFlavorPropertiesActual1 := make(chan *types.Any, 1)
	inputFlavorProperties1 := types.AnyString(`{"flavor":"zookeeper","role":"leader"}`)
	inputInstanceSpecActual1 := make(chan instance.Spec, 1)
	inputInstanceSpec1 := instance.Spec{
		Properties: inputFlavorProperties1,
		Tags:       map[string]string{"foo": "bar1"},
	}

	inputFlavorPropertiesActual2 := make(chan *types.Any, 1)
	inputFlavorProperties2 := types.AnyString(`{"flavor":"zookeeper","role":"follower"}`)
	inputInstanceSpecActual2 := make(chan instance.Spec, 1)
	inputInstanceSpec2 := instance.Spec{
		Properties: inputFlavorProperties2,
		Tags:       map[string]string{"foo": "bar2"},
	}

	inputInstanceIndexActual1 := make(chan group.Index, 1)
	inputInstanceIndexActual2 := make(chan group.Index, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testing_flavor.Plugin{
				DoPrepare: func(
					flavorProperties *types.Any,
					instanceSpec instance.Spec,
					allocation group.AllocationMethod,
					idx group.Index) (instance.Spec, error) {

					inputFlavorPropertiesActual1 <- flavorProperties
					inputInstanceSpecActual1 <- instanceSpec
					inputInstanceIndexActual1 <- idx
					return instanceSpec, nil
				},
			},
			"type2": &testing_flavor.Plugin{
				DoPrepare: func(
					flavorProperties *types.Any,
					instanceSpec instance.Spec,
					allocation group.AllocationMethod,
					idx group.Index) (instance.Spec, error) {

					inputFlavorPropertiesActual2 <- flavorProperties
					inputInstanceSpecActual2 <- instanceSpec
					inputInstanceIndexActual2 <- idx

					return instanceSpec, errors.New("bad-thing-happened")
				},
			},
		},
	))
	require.NoError(t, err)

	spec, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Prepare(
		inputFlavorProperties1,
		inputInstanceSpec1,
		allocation,
		index)
	require.NoError(t, err)
	require.Equal(t, inputInstanceSpec1, spec)

	_, err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Prepare(
		inputFlavorProperties2,
		inputInstanceSpec2,
		allocation,
		index)
	require.Error(t, err)
	require.Equal(t, "bad-thing-happened", err.Error())

	server.Stop()

	require.Equal(t, inputFlavorProperties1, <-inputFlavorPropertiesActual1)
	require.Equal(t, inputInstanceSpec1, <-inputInstanceSpecActual1)
	require.Equal(t, index, <-inputInstanceIndexActual1)

	require.Equal(t, inputFlavorProperties2, <-inputFlavorPropertiesActual2)
	require.Equal(t, inputInstanceSpec2, <-inputInstanceSpecActual2)
	require.Equal(t, index, <-inputInstanceIndexActual2)
}

func TestFlavorMultiPluginHealthy(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual1 := make(chan *types.Any, 1)
	inputInstanceActual1 := make(chan instance.Description, 1)
	inputProperties1 := types.AnyString("{}")
	inputInstance1 := instance.Description{
		ID:   instance.ID("foo1"),
		Tags: map[string]string{"foo": "bar1"},
	}

	inputPropertiesActual2 := make(chan *types.Any, 1)
	inputInstanceActual2 := make(chan instance.Description, 1)
	inputProperties2 := types.AnyString("{}")
	inputInstance2 := instance.Description{
		ID:   instance.ID("foo2"),
		Tags: map[string]string{"foo": "bar2"},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testing_flavor.Plugin{
				DoHealthy: func(properties *types.Any, inst instance.Description) (flavor.Health, error) {
					inputPropertiesActual1 <- properties
					inputInstanceActual1 <- inst
					return flavor.Healthy, nil
				},
			},
			"type2": &testing_flavor.Plugin{
				DoHealthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
					inputPropertiesActual2 <- flavorProperties
					inputInstanceActual2 <- inst
					return flavor.Unknown, errors.New("oh-noes")
				},
			},
		}))
	require.NoError(t, err)

	health, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Healthy(inputProperties1, inputInstance1)
	require.NoError(t, err)
	require.Equal(t, flavor.Healthy, health)

	_, err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Healthy(inputProperties2, inputInstance2)
	require.Error(t, err)
	require.Equal(t, "oh-noes", err.Error())

	require.Equal(t, inputProperties1, <-inputPropertiesActual1)
	require.Equal(t, inputInstance1, <-inputInstanceActual1)

	require.Equal(t, inputProperties2, <-inputPropertiesActual2)
	require.Equal(t, inputInstance2, <-inputInstanceActual2)

	server.Stop()
}

func TestFlavorMultiPluginDrain(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual1 := make(chan *types.Any, 1)
	inputInstanceActual1 := make(chan instance.Description, 1)
	inputProperties1 := types.AnyString("{}")
	inputInstance1 := instance.Description{
		ID:   instance.ID("foo1"),
		Tags: map[string]string{"foo": "bar1"},
	}

	inputPropertiesActual2 := make(chan *types.Any, 1)
	inputInstanceActual2 := make(chan instance.Description, 1)
	inputProperties2 := types.AnyString("{}")
	inputInstance2 := instance.Description{
		ID:   instance.ID("foo2"),
		Tags: map[string]string{"foo": "bar2"},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testing_flavor.Plugin{
				DoDrain: func(properties *types.Any, inst instance.Description) error {
					inputPropertiesActual1 <- properties
					inputInstanceActual1 <- inst
					return nil
				},
			},
			"type2": &testing_flavor.Plugin{
				DoDrain: func(flavorProperties *types.Any, inst instance.Description) error {
					inputPropertiesActual2 <- flavorProperties
					inputInstanceActual2 <- inst
					return errors.New("oh-noes")
				},
			},
		},
	))
	require.NoError(t, err)

	require.NoError(t, must(NewClient(plugin.Name(name+"/type1"), socketPath)).Drain(inputProperties1, inputInstance1))

	require.Equal(t, inputProperties1, <-inputPropertiesActual1)
	require.Equal(t, inputInstance1, <-inputInstanceActual1)

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Drain(inputProperties2, inputInstance2)
	require.Error(t, err)
	require.Equal(t, "oh-noes", err.Error())

	require.Equal(t, inputProperties2, <-inputPropertiesActual2)
	require.Equal(t, inputInstance2, <-inputInstanceActual2)

	server.Stop()
}
