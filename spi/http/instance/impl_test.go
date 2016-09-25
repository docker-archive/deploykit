package instance

import (
	"encoding/json"
	"testing"

	"github.com/docker/libmachete/plugin/util"
	"github.com/docker/libmachete/spi/instance"
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
	return t.DescribeInstances(tags)
}

func TestInstancePluginValidate(t *testing.T) {

	listen := "tcp://:4321"

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
