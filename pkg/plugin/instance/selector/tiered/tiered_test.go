package tiered

import (
	"fmt"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/plugin/instance/selector/internal"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type fakeDiscovery map[string]*plugin.Endpoint

func (f fakeDiscovery) List() (map[string]*plugin.Endpoint, error) {
	return f, nil
}
func (f fakeDiscovery) Find(n plugin.Name) (*plugin.Endpoint, error) {
	lookup, _ := n.GetLookupAndType()
	return f[lookup], nil
}

func TestTieredChoices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	choices := []selector.Choice{
		// on-prem
		{
			Name: plugin.Name("on-prem-vsphere/compute"),
		},
		// spot instance
		{
			Name: plugin.Name("aws/ec2_spot_instance"),
		},
		// on-demand instance
		{
			Name: plugin.Name("aws/ec2_instance"),
		},
	}

	// mock plugins
	vsphere := mock_instance.NewMockPlugin(ctrl)
	spot := mock_instance.NewMockPlugin(ctrl)
	ondemand := mock_instance.NewMockPlugin(ctrl)

	selector := NewPlugin(
		func() discovery.Plugins {
			return fakeDiscovery(map[string]*plugin.Endpoint{})
		}, choices)
	selector.(*impl).Plugin.(*internal.Base).PluginClientFunc =
		func(name plugin.Name) (instance.Plugin, error) {
			switch name {
			case plugin.Name("on-prem-vsphere/compute"):
				return vsphere, nil
			case plugin.Name("aws/ec2_spot_instance"):
				return spot, nil
			case plugin.Name("aws/ec2_instance"):
				return ondemand, nil
			}
			return nil, nil
		}

	// input to the plugin should be a map containing the plugins and their
	// properties

	properties := map[string]*types.Any{}

	require.NoError(t, types.AnyYAMLMust([]byte(`
on-prem-vsphere/compute :
  datacenter: vs1
  esx: host1

aws/ec2_spot_instance:
  region: us-east-1
  bid: 0.02
  type: m2xlarge

aws/ec2_instance:
  region: us-east-1
  type: m2xlarge

`)).Decode(&properties))

	// build the input to Provision
	request := instance.Spec{
		Properties: types.AnyValueMust(properties),
	}

	// make everyone return an error and verify the input
	vsphere.EXPECT().Provision(gomock.Any()).Do(
		func(spec instance.Spec) (*instance.ID, error) {
			require.Equal(t, spec.Properties, types.AnyValueMust(
				map[string]interface{}{
					"datacenter": "vs1",
					"esx":        "host1",
				}))
			return nil, nil

		},
	).Return(nil, fmt.Errorf("can't do"))

	spot.EXPECT().Provision(gomock.Any()).Do(
		func(spec instance.Spec) (*instance.ID, error) {
			require.Equal(t, spec.Properties, types.AnyValueMust(
				map[string]interface{}{
					"region": "us-east-1",
					"bid":    0.02,
					"type":   "m2xlarge",
				}))
			return nil, nil
		},
	).Return(nil, fmt.Errorf("can't do"))

	ondemand.EXPECT().Provision(gomock.Any()).Do(
		func(spec instance.Spec) (*instance.ID, error) {
			require.Equal(t, spec.Properties, types.AnyValueMust(
				map[string]interface{}{
					"region": "us-east-1",
					"type":   "m2xlarge",
				}))
			return nil, nil
		},
	).Return(nil, fmt.Errorf("can't do"))

	_, err := selector.Provision(request)
	require.Error(t, err)

	provisioned := instance.ID("spot1")

	// Now have a success in provisioning
	vsphere.EXPECT().Provision(gomock.Any()).Return(nil, fmt.Errorf("can't do"))
	spot.EXPECT().Provision(gomock.Any()).Do(
		func(spec instance.Spec) (*instance.ID, error) {
			require.Equal(t, spec.Properties, types.AnyValueMust(
				map[string]interface{}{
					"region": "us-east-1",
					"bid":    0.02,
					"type":   "m2xlarge",
				}))
			return nil, nil
		},
	).Return(&provisioned, nil)

	new, err := selector.Provision(request)
	require.NoError(t, err)
	require.Equal(t, provisioned, *new)

	// Missing config
	delete(properties, "on-prem-vsphere/compute")

	_, err = selector.Provision(instance.Spec{
		Properties: types.AnyValueMust(properties),
	})
	require.Error(t, err)

}
