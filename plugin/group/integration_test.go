package scaler

import (
	"encoding/json"
	"fmt"
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const (
	id         = group.ID("testGroup")
	pluginName = "test"
)

var config = group.Configuration{
	ID:         id,
	Properties: propertiesConfig(3, "data"),
}

func propertiesConfig(instances int, data string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
	  "Size": %d,
	  "InstancePlugin": "test",
	  "InstancePluginProperties": {
	    "OpaqueValue": "%s"
	  }
	}`, instances, data))
}

func mockedPluginGroup(ctrl *gomock.Controller) (*mock_instance.MockPlugin, group.Plugin) {
	plugin := mock_instance.NewMockPlugin(ctrl)
	// TODO(wfarner): Wire a 'ticker factory' through to take the time element out of tests, and allow tests
	// to define when state course-correction happens.
	grp := NewGroup(map[string]instance.Plugin{pluginName: plugin}, 1*time.Hour)
	return plugin, grp
}

func TestInvalidGroupCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	_, grp := mockedPluginGroup(ctrl)

	require.Error(t, grp.DestroyGroup(id))
	_, err := grp.InspectGroup(id)
	require.Error(t, err)
	require.Error(t, grp.UnwatchGroup(id))
	_, err = grp.DescribeUpdate(config)
	require.Error(t, err)
	require.Error(t, grp.UpdateGroup(config))
}

func instanceProperties(config group.Configuration) json.RawMessage {
	groupProperties := map[string]json.RawMessage{}
	err := json.Unmarshal(config.Properties, &groupProperties)
	if err != nil {
		panic(err)
	}
	return groupProperties["InstancePluginProperties"]
}

func expectValidate(plugin *mock_instance.MockPlugin, config group.Configuration) *gomock.Call {
	return plugin.EXPECT().Validate(instanceProperties(config))
}

func expectDescribe(plugin *mock_instance.MockPlugin, id group.ID) *gomock.Call {
	return plugin.EXPECT().DescribeInstances(map[string]string{groupTag: string(id)})
}

func fakeInstance(config group.Configuration, id string) instance.Description {
	return instance.Description{
		ID: instance.ID(id),
		Tags: map[string]string{
			groupTag:  string(config.ID),
			configTag: instanceConfigHash(instanceProperties(config)),
			"other":   "ignored",
		},
	}
}

func TestNoopUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	expectValidate(plugin, config).Return(nil).Times(2)
	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil).Times(2)

	desc, err := grp.DescribeUpdate(config)
	require.NoError(t, err)
	require.Equal(t, "Noop", desc)

	require.NoError(t, grp.UpdateGroup(config))
}

func TestRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	updated := group.Configuration{
		ID:         id,
		Properties: propertiesConfig(3, "data2"),
	}

	expectValidate(plugin, updated).Return(nil)
	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil)

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(t, "Performs a rolling update on 3 instances", desc)

	// TODO(wfarner): This test is slow due to sleeps during the rolling update.  This will
	// be addressed when updates are gated based on 'health' rather than arbitrary time.

	// TODO(wfarner): Since the scaler is effectively disabled, instances are not created during this rolling
	// update.  Once the scaler can be predictably controlled from tests, 'tick' it to induce creation of instances.

	expectValidate(plugin, updated).Return(nil)
	gomock.InOrder(
		expectDescribe(plugin, id).Return([]instance.Description{
			fakeInstance(config, "a"),
			fakeInstance(config, "b"),
			fakeInstance(config, "c"),
		}, nil),
		expectDescribe(plugin, id).Return([]instance.Description{
			fakeInstance(config, "a"),
			fakeInstance(config, "b"),
			fakeInstance(config, "c"),
		}, nil),
		plugin.EXPECT().Destroy(instance.ID("a")).Return(nil),
		expectDescribe(plugin, id).Return([]instance.Description{
			fakeInstance(config, "b"),
			fakeInstance(config, "c"),
		}, nil),
		plugin.EXPECT().Destroy(instance.ID("b")).Return(nil),
		expectDescribe(plugin, id).Return([]instance.Description{
			fakeInstance(config, "c"),
		}, nil),
		plugin.EXPECT().Destroy(instance.ID("c")).Return(nil),
		expectDescribe(plugin, id).Return([]instance.Description{}, nil),
	)
	require.NoError(t, grp.UpdateGroup(updated))
}

func TestRollAndAdjustScale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	updated := group.Configuration{
		ID:         id,
		Properties: propertiesConfig(8, "data2"),
	}

	expectValidate(plugin, updated).Return(nil)
	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil)

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(
		t,
		"Performs a rolling update on 3 instances, then adds 5 instances to increase the group size to 8",
		desc)
}

func TestScaleIncrease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	newConfig := group.Configuration{
		ID:         id,
		Properties: propertiesConfig(6, "data"),
	}

	expectValidate(plugin, newConfig).Return(nil)
	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil)

	desc, err := grp.DescribeUpdate(newConfig)
	require.NoError(t, err)
	require.Equal(t, "Adds 3 instances to increase the group size to 6", desc)

	// TODO(wfarner): Tick scaler and observe 3 instances created.
}

func TestScaleDecrease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	newConfig := group.Configuration{
		ID:         id,
		Properties: propertiesConfig(1, "data"),
	}

	expectValidate(plugin, newConfig).Return(nil)
	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil)

	desc, err := grp.DescribeUpdate(newConfig)
	require.NoError(t, err)
	require.Equal(t, "Terminates 2 instances to reduce the group size to 1", desc)

	// TODO(wfarner): Tick scaler and observe 3 instances created.
}

func TestUnwatchGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	require.NoError(t, grp.UnwatchGroup(id))
}

func TestDestroyGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, config).Return(nil)
	require.NoError(t, grp.WatchGroup(config))

	expectDescribe(plugin, id).Return([]instance.Description{
		fakeInstance(config, "a"),
		fakeInstance(config, "b"),
		fakeInstance(config, "c"),
	}, nil)
	plugin.EXPECT().Destroy(instance.ID("a")).Return(nil)
	plugin.EXPECT().Destroy(instance.ID("b")).Return(nil)
	plugin.EXPECT().Destroy(instance.ID("c")).Return(nil)

	require.NoError(t, grp.DestroyGroup(id))
}
