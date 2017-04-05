package flavor

import (
	"errors"
	"testing"

	mock_flavor "github.com/docker/infrakit.gcp/mock/flavor"
	mock_gcloud "github.com/docker/infrakit.gcp/mock/gcloud"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/compute/v1"
)

func logicalID(v string) *instance.LogicalID {
	id := instance.LogicalID(v)
	return &id
}

var inst = instance.Spec{
	Properties:  types.AnyString("{}"),
	Tags:        map[string]string{},
	Init:        "",
	LogicalID:   logicalID("id"),
	Attachments: []instance.Attachment{{ID: "att1", Type: "nic"}},
}

func pluginLookup(plugins map[string]flavor.Plugin) group.FlavorPluginLookup {
	return func(key plugin.Name) (flavor.Plugin, error) {
		plugin, has := plugins[key.String()]
		if has {
			return plugin, nil
		}
		return nil, errors.New("Plugin doesn't exist")
	}
}

func NewMockGCloud(t *testing.T) (*mock_gcloud.MockAPI, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mock_gcloud.NewMockAPI(ctrl), ctrl
}

func NewFlavorPlugin(api gcloud.API) flavor.Plugin {
	return &flavorCombo{api, nil}
}

func TestMergeBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := mock_flavor.NewMockPlugin(ctrl)
	b := mock_flavor.NewMockPlugin(ctrl)

	plugins := map[string]flavor.Plugin{"a": a, "b": b}

	combo := flavorCombo{nil, pluginLookup(plugins)}

	properties := types.AnyString(`{
	  "Flavors": [
	    {
	      "Plugin": "a",
	      "Properties": {"a": "1"}
	    },
	    {
	      "Plugin": "b",
	      "Properties": {"b": "2"}
	    }
	  ]
	}`)

	allocation := group_types.AllocationMethod{Size: 1}
	index := group_types.Index{}
	a.EXPECT().Prepare(types.AnyString(`{"a": "1"}`), inst, allocation, index).Return(instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"a": "1", "c": "4"},
		Init:        "init data a",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "a", Type: "nic"}},
	}, nil)

	b.EXPECT().Prepare(types.AnyString(`{"b": "2"}`), inst, allocation, index).Return(instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"b": "2", "c": "5"},
		Init:        "init data b",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "b", Type: "gpu"}},
	}, nil)

	result, err := combo.Prepare(properties, inst, group_types.AllocationMethod{Size: 1}, index)
	require.NoError(t, err)

	expected := instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"a": "1", "b": "2", "c": "5"},
		Init:        "init data a\ninit data b",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "att1", Type: "nic"}, {ID: "a", Type: "nic"}, {ID: "b", Type: "gpu"}},
	}
	require.Equal(t, expected, result)
}

func TestMergeNoLogicalID(t *testing.T) {
	// Tests regression of a bug where a zero value was returned for the LogicalID despite nil being passed.

	inst = instance.Spec{
		Properties:  types.AnyString("{}"),
		Tags:        map[string]string{},
		Init:        "",
		Attachments: []instance.Attachment{{ID: "att1", Type: "nic"}},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	a := mock_flavor.NewMockPlugin(ctrl)
	b := mock_flavor.NewMockPlugin(ctrl)

	plugins := map[string]flavor.Plugin{"a": a, "b": b}

	combo := flavorCombo{nil, pluginLookup(plugins)}

	properties := types.AnyString(`{
	  "Flavors": [
	    {
	      "Plugin": "a",
	      "Properties": {"a": "1"}
	    },
	    {
	      "Plugin": "b",
	      "Properties": {"b": "2"}
	    }
	  ]
	}`)

	allocation := group_types.AllocationMethod{Size: 1}
	index := group_types.Index{}

	a.EXPECT().Prepare(types.AnyString(`{"a": "1"}`), inst, allocation, index).Return(instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"a": "1", "c": "4"},
		Init:        "init data a",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "a", Type: "nic"}},
	}, nil)

	b.EXPECT().Prepare(types.AnyString(`{"b": "2"}`), inst, allocation, index).Return(instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"b": "2", "c": "5"},
		Init:        "init data b",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "b", Type: "gpu"}},
	}, nil)

	result, err := combo.Prepare(properties, inst, group_types.AllocationMethod{Size: 1}, index)
	require.NoError(t, err)

	expected := instance.Spec{
		Properties:  inst.Properties,
		Tags:        map[string]string{"a": "1", "b": "2", "c": "5"},
		Init:        "init data a\ninit data b",
		LogicalID:   inst.LogicalID,
		Attachments: []instance.Attachment{{ID: "att1", Type: "nic"}, {ID: "a", Type: "nic"}, {ID: "b", Type: "gpu"}},
	}
	require.Equal(t, expected, result)
}

func TestHealthy(t *testing.T) {
	var tests = []struct {
		status         string
		expectedHealth flavor.Health
		expectedError  error
	}{
		{"STOPPED", flavor.Unhealthy, nil},
		{"STOPPING", flavor.Unhealthy, nil},
		{"SUSPENDED", flavor.Unhealthy, nil},
		{"SUSPENDING", flavor.Unhealthy, nil},
		{"TERMINATED", flavor.Unhealthy, nil},
		{"RUNNING", flavor.Healthy, nil},
		{"PROVISIONING", flavor.Unknown, nil},
		{"STAGING", flavor.Unknown, nil},
		{"", flavor.Unknown, nil},
	}

	for _, test := range tests {
		api, _ := NewMockGCloud(t)
		api.EXPECT().GetInstance("vm-1").Return(&compute.Instance{Status: test.status}, nil)

		plugin := NewFlavorPlugin(api)
		health, err := plugin.Healthy(nil, instance.Description{
			ID: "vm-1",
		})

		require.Equal(t, test.expectedHealth, health, test.status)
		require.Equal(t, test.expectedError, err)
	}
}
