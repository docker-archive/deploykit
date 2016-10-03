package group

import (
	"encoding/json"
	"fmt"
	mock_instance "github.com/docker/infrakit/mock/spi/instance"
	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/group"
	"github.com/docker/infrakit/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const (
	id         = group.ID("testGroup")
	pluginName = "test"
)

var (
	minions = group.Spec{
		ID:         id,
		Properties: minionProperties(3, "data", "init"),
	}

	leaders = group.Spec{
		ID:         id,
		Properties: leaderProperties(leaderIDs, "data"),
	}

	leaderIDs = []instance.LogicalID{"192.168.0.4", "192.168.0.5", "192.168.0.6"}
)

func flavorPluginLookup(_ string) (flavor.Plugin, error) {
	return &testFlavor{}, nil
}

func minionProperties(instances int, instanceData string, flavorInit string) *json.RawMessage {
	r := json.RawMessage(fmt.Sprintf(`{
	  "Instance" : {
              "Plugin": "test",
	      "Properties": {
	          "OpaqueValue": "%s"
	      }
          },
	  "Flavor" : {
              "Plugin" : "test",
	      "Properties": {
	          "Type": "minion",
	          "Size": %d,
	          "Init": "%s"
	      }
          }
	}`, instanceData, instances, flavorInit))
	return &r
}

func leaderProperties(logicalIDs []instance.LogicalID, data string) *json.RawMessage {
	idsValue, err := json.Marshal(logicalIDs)
	if err != nil {
		panic(err)
	}

	r := json.RawMessage(fmt.Sprintf(`{
	  "Instance" : {
              "Plugin": "test",
	      "Properties": {
	          "OpaqueValue": "%s"
	      }
          },
	  "Flavor" : {
              "Plugin": "test",
	      "Properties": {
	         "Type": "leader",
	         "Shards": %s
	      }
          }
	}`, data, idsValue))
	return &r
}

func fakeInstancePluginLookup(pluginName string, plugin instance.Plugin) InstancePluginLookup {
	return func(key string) (instance.Plugin, error) {
		if key == pluginName {
			return plugin, nil
		}
		return nil, nil
	}
}

func mockedPluginGroup(ctrl *gomock.Controller) (*mock_instance.MockPlugin, group.Plugin) {
	plugin := mock_instance.NewMockPlugin(ctrl)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)
	return plugin, grp
}

func TestInvalidGroupCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	require.Error(t, grp.DestroyGroup(id))
	_, err := grp.InspectGroup(id)
	require.Error(t, err)
	require.Error(t, grp.UnwatchGroup(id))
	require.Error(t, grp.StopUpdate(id))
	expectValidate(plugin, minions).Return(nil).MinTimes(1)
	_, err = grp.DescribeUpdate(minions)
	require.Error(t, err)
	require.Error(t, grp.UpdateGroup(minions))
}

func instanceProperties(config group.Spec) json.RawMessage {
	spec := types.Spec{}
	err := json.Unmarshal(*config.Properties, &spec)
	if err != nil {
		panic(err)
	}
	return *spec.Instance.Properties
}

func expectValidate(plugin *mock_instance.MockPlugin, config group.Spec) *gomock.Call {
	return plugin.EXPECT().Validate(instanceProperties(config))
}

func memberTags(id group.ID) map[string]string {
	return map[string]string{groupTag: string(id)}
}

func provisionTags(config group.Spec) map[string]string {
	tags := memberTags(config.ID)
	tags[configTag] = types.MustParse(types.ParseProperties(config)).InstanceHash()

	return tags
}

func newFakeInstance(config group.Spec, logicalID *instance.LogicalID) instance.Spec {
	// Inject another tag to simulate instances being tagged out-of-band.  Our implementation should ignore tags
	// we did not create.
	tags := map[string]string{"other": "ignored"}
	for k, v := range provisionTags(config) {
		tags[k] = v
	}

	return instance.Spec{
		LogicalID: logicalID,
		Tags:      provisionTags(config),
	}
}

func TestNoopUpdate(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	desc, err := grp.DescribeUpdate(minions)
	require.NoError(t, err)
	require.Equal(t, "Noop", desc)

	require.NoError(t, grp.UpdateGroup(minions))

	instances, err := plugin.DescribeInstances(memberTags(minions.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, newFakeInstance(minions, nil).Tags, i.Tags)
	}
}

func TestRollingUpdate(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data2", "flavor2")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(t, "Performs a rolling update on 3 instances", desc)

	require.NoError(t, grp.UpdateGroup(updated))

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}
}

func TestRollAndAdjustScale(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data2", "flavor2")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(
		t,
		"Performs a rolling update on 3 instances, then adds 5 instances to increase the group size to 8",
		desc)

	require.NoError(t, grp.UpdateGroup(updated))

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}
}

func TestScaleIncrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data", "init")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(t, "Adds 5 instances to increase the group size to 8", desc)

	require.NoError(t, grp.UpdateGroup(updated))

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}
}

func TestScaleDecrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(1, "data", "flavorInit")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(t, "Terminates 2 instances to reduce the group size to 1", desc)

	require.NoError(t, grp.UpdateGroup(updated))

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) <= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}
}

func TestUnwatchGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	expectValidate(plugin, minions).Return(nil)
	require.NoError(t, grp.WatchGroup(minions))

	require.NoError(t, grp.UnwatchGroup(id))
}

func TestDestroyGroup(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))
	require.NoError(t, grp.DestroyGroup(minions.ID))

	instances, err := plugin.DescribeInstances(memberTags(minions.ID))
	require.NoError(t, err)
	require.Equal(t, 0, len(instances))
}

func TestSuperviseQuorum(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(leaders, &leaderIDs[0]),
		newFakeInstance(leaders, &leaderIDs[1]),
		newFakeInstance(leaders, &leaderIDs[2]),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(leaders))

	updated := group.Spec{ID: id, Properties: leaderProperties(leaderIDs, "data2")}

	time.Sleep(1 * time.Second)

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)
	require.Equal(t, "Performs a rolling update on 3 instances", desc)

	require.NoError(t, grp.UpdateGroup(updated))

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	// TODO(wfarner): Validate logical IDs in created instances.
}

func TestUpdateCompletes(t *testing.T) {
	// Tests that a completed update clears the 'update in progress state', allowing another update to commence.

	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data", "init")}
	require.NoError(t, grp.UpdateGroup(updated))

	updated = group.Spec{ID: id, Properties: minionProperties(5, "data", "init")}
	require.NoError(t, grp.UpdateGroup(updated))
}

func TestInstanceAndFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data2", "updated init")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)

	require.Equal(t, "Performs a rolling update on 3 instances", desc)

	require.NoError(t, grp.UpdateGroup(updated))

	for _, inst := range plugin.instancesCopy() {
		require.Equal(t, "updated init", inst.Init)

		properties := map[string]string{}
		err = json.Unmarshal(types.RawMessage(inst.Properties), &properties)
		require.NoError(t, err)
		require.Equal(t, "data2", properties["OpaqueValue"])
	}
}

func TestFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data", "updated init")}

	desc, err := grp.DescribeUpdate(updated)
	require.NoError(t, err)

	// TODO(wfarner): Update this test when addressing the TODO in Spec.InstanceHash().
	require.Equal(t, "Noop", desc)
}
