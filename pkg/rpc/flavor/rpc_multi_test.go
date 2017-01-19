package flavor

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
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

	inputFlavorPropertiesActual1 := make(chan json.RawMessage, 1)
	inputFlavorProperties1 := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))

	inputFlavorPropertiesActual2 := make(chan json.RawMessage, 1)
	inputFlavorProperties2 := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"follower"}`))

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testPlugin{
				DoValidate: func(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
					inputFlavorPropertiesActual1 <- flavorProperties
					return nil
				},
			},
			"type2": &testPlugin{
				DoValidate: func(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
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

	inputFlavorPropertiesActual1 := make(chan json.RawMessage, 1)
	inputFlavorProperties1 := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))
	inputInstanceSpecActual1 := make(chan instance.Spec, 1)
	inputInstanceSpec1 := instance.Spec{
		Properties: &inputFlavorProperties1,
		Tags:       map[string]string{"foo": "bar1"},
	}

	inputFlavorPropertiesActual2 := make(chan json.RawMessage, 1)
	inputFlavorProperties2 := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"follower"}`))
	inputInstanceSpecActual2 := make(chan instance.Spec, 1)
	inputInstanceSpec2 := instance.Spec{
		Properties: &inputFlavorProperties2,
		Tags:       map[string]string{"foo": "bar2"},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testPlugin{
				DoPrepare: func(
					flavorProperties json.RawMessage,
					instanceSpec instance.Spec,
					allocation types.AllocationMethod) (instance.Spec, error) {

					inputFlavorPropertiesActual1 <- flavorProperties
					inputInstanceSpecActual1 <- instanceSpec

					return instanceSpec, nil
				},
			},
			"type2": &testPlugin{
				DoPrepare: func(
					flavorProperties json.RawMessage,
					instanceSpec instance.Spec,
					allocation types.AllocationMethod) (instance.Spec, error) {

					inputFlavorPropertiesActual2 <- flavorProperties
					inputInstanceSpecActual2 <- instanceSpec

					return instanceSpec, errors.New("bad-thing-happened")
				},
			},
		},
	))
	require.NoError(t, err)

	spec, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).Prepare(
		inputFlavorProperties1,
		inputInstanceSpec1,
		allocation)
	require.NoError(t, err)
	require.Equal(t, inputInstanceSpec1, spec)

	_, err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Prepare(
		inputFlavorProperties2,
		inputInstanceSpec2,
		allocation)
	require.Error(t, err)
	require.Equal(t, "bad-thing-happened", err.Error())

	server.Stop()

	require.Equal(t, inputFlavorProperties1, <-inputFlavorPropertiesActual1)
	require.Equal(t, inputInstanceSpec1, <-inputInstanceSpecActual1)

	require.Equal(t, inputFlavorProperties2, <-inputFlavorPropertiesActual2)
	require.Equal(t, inputInstanceSpec2, <-inputInstanceSpecActual2)
}

func TestFlavorMultiPluginHealthy(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inputPropertiesActual1 := make(chan json.RawMessage, 1)
	inputInstanceActual1 := make(chan instance.Description, 1)
	inputProperties1 := json.RawMessage("{}")
	inputInstance1 := instance.Description{
		ID:   instance.ID("foo1"),
		Tags: map[string]string{"foo": "bar1"},
	}

	inputPropertiesActual2 := make(chan json.RawMessage, 1)
	inputInstanceActual2 := make(chan instance.Description, 1)
	inputProperties2 := json.RawMessage("{}")
	inputInstance2 := instance.Description{
		ID:   instance.ID("foo2"),
		Tags: map[string]string{"foo": "bar2"},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testPlugin{
				DoHealthy: func(properties json.RawMessage, inst instance.Description) (flavor.Health, error) {
					inputPropertiesActual1 <- properties
					inputInstanceActual1 <- inst
					return flavor.Healthy, nil
				},
			},
			"type2": &testPlugin{
				DoHealthy: func(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
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

	inputPropertiesActual1 := make(chan json.RawMessage, 1)
	inputInstanceActual1 := make(chan instance.Description, 1)
	inputProperties1 := json.RawMessage("{}")
	inputInstance1 := instance.Description{
		ID:   instance.ID("foo1"),
		Tags: map[string]string{"foo": "bar1"},
	}

	inputPropertiesActual2 := make(chan json.RawMessage, 1)
	inputInstanceActual2 := make(chan instance.Description, 1)
	inputProperties2 := json.RawMessage("{}")
	inputInstance2 := instance.Description{
		ID:   instance.ID("foo2"),
		Tags: map[string]string{"foo": "bar2"},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]flavor.Plugin{
			"type1": &testPlugin{
				DoDrain: func(properties json.RawMessage, inst instance.Description) error {
					inputPropertiesActual1 <- properties
					inputInstanceActual1 <- inst
					return nil
				},
			},
			"type2": &testPlugin{
				DoDrain: func(flavorProperties json.RawMessage, inst instance.Description) error {
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
