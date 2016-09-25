package flavor

import (
	"encoding/json"
	"testing"

	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/plugin/util"
	"github.com/docker/libmachete/spi/flavor"
	"github.com/docker/libmachete/spi/instance"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	DoValidate     func(flavorProperties json.RawMessage, parsed types.Schema) (flavor.InstanceIDKind, error)
	DoPreProvision func(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error)
	DoHealthy      func(inst instance.Description) (bool, error)
}

func (t *testPlugin) Validate(flavorProperties json.RawMessage, parsed types.Schema) (flavor.InstanceIDKind, error) {
	return t.DoValidate(flavorProperties, parsed)
}
func (t *testPlugin) PreProvision(flavorProperties json.RawMessage, spec instance.Spec) (instance.Spec, error) {
	return t.DoPreProvision(flavorProperties, spec)
}
func (t *testPlugin) Healthy(inst instance.Description) (bool, error) {
	return t.DoHealthy(inst)
}

func TestFlavorPluginValidate(t *testing.T) {
	listen := "tcp://:4322"

	inputFlavorPropertiesActual := make(chan json.RawMessage, 1)
	inputFlavorProperties := json.RawMessage([]byte(`{"flavor":"zookeeper","role":"leader"}`))
	inputGroupSpecActual := make(chan types.Schema, 1)
	inputGroupSpec := types.Schema{
		Size:                   1,
		LogicalIDs:             []instance.LogicalID{instance.LogicalID("overlord")},
		FlavorPlugin:           "zookeeper",
		FlavorPluginProperties: &inputFlavorProperties,
	}

	instanceIDKind := flavor.IDKindPhysical

	stop, _, err := util.StartServer(listen, PluginServer(&testPlugin{
		DoValidate: func(flavorProperties json.RawMessage, groupSpec types.Schema) (flavor.InstanceIDKind, error) {

			t.Log("Received:", string(flavorProperties), groupSpec)
			inputFlavorPropertiesActual <- flavorProperties
			inputGroupSpecActual <- groupSpec

			return instanceIDKind, nil
		},
	}))
	require.NoError(t, err)

	callable, err := util.NewClient(listen)
	require.NoError(t, err)

	flavorPluginClient := PluginClient(callable)

	// Make call
	kind, err := flavorPluginClient.Validate(inputFlavorProperties, inputGroupSpec)
	require.NoError(t, err)
	require.Equal(t, instanceIDKind, kind)

	close(stop)

	require.Equal(t, inputFlavorProperties, <-inputFlavorPropertiesActual)
	require.Equal(t, inputGroupSpec, <-inputGroupSpecActual)
}
