package group

import (
	"encoding/json"
	"fmt"
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/flavor"
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

var (
	minions = group.Configuration{
		ID:         id,
		Role:       "minions",
		Properties: minionProperties(3, "data"),
	}

	leaders = group.Configuration{
		ID:         id,
		Role:       "leaders",
		Properties: leaderProperties(managerIPs, "data"),
	}

	managerIPs = []string{"192.168.0.4", "192.168.0.5", "192.168.0.6"}

	flavorPlugin = testFlavor{tags: map[string]string{"custom": "value"}}
)

func flavorPluginLookup(_ string) (flavor.Plugin, error) {
	return &flavorPlugin, nil
}

func minionProperties(instances int, data string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
	  "Size": %d,
	  "InstancePlugin": "test",
	  "InstancePluginProperties": {
	    "OpaqueValue": "%s"
	  }
	}`, instances, data))
}

func leaderProperties(ips []string, data string) json.RawMessage {
	ipValue, err := json.Marshal(ips)
	if err != nil {
		panic(err)
	}

	return json.RawMessage(fmt.Sprintf(`{
	  "IPs": %s,
	  "InstancePlugin": "test",
	  "InstancePluginProperties": {
	    "OpaqueValue": "%s"
	  }
	}`, ipValue, data))
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

func memberTags(id group.ID) map[string]string {
	return map[string]string{groupTag: string(id)}
}

func expectDescribe(plugin *mock_instance.MockPlugin, id group.ID) *gomock.Call {
	return plugin.EXPECT().DescribeInstances(memberTags(id))
}

func provisionTags(config group.Configuration) map[string]string {
	tags := memberTags(config.ID)
	tags[configTag] = types.MustParse(types.ParseProperties(config)).InstanceHash()

	for k, v := range flavorPlugin.tags {
		tags[k] = v
	}

	return tags
}

func createFake(config group.Configuration, id string) instance.Description {
	tags := provisionTags(config)
	// Inject another tag to simulate instances being tagged out-of-band.  Our implementation should ignore tags
	// we did not create.
	tags["other"] = "ignored"

	return instance.Description{ID: instance.ID(id), Tags: tags}
}

func TestNoopUpdate(t *testing.T) {
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
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
		require.Equal(t, provisionTags(minions), i.Tags)
	}
}

func TestRollingUpdate(t *testing.T) {
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Configuration{ID: id, Role: minions.Role, Properties: minionProperties(3, "data2")}

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
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Configuration{ID: id, Role: minions.Role, Properties: minionProperties(8, "data2")}

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
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Configuration{ID: id, Role: minions.Role, Properties: minionProperties(8, "data")}

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
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Configuration{ID: id, Role: minions.Role, Properties: minionProperties(1, "data")}

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
	plugin := NewTestInstancePlugin(
		provisionTags(minions),
		provisionTags(minions),
		provisionTags(minions),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(minions))
	require.NoError(t, grp.DestroyGroup(minions.ID))

	instances, err := plugin.DescribeInstances(memberTags(minions.ID))
	require.NoError(t, err)
	require.Equal(t, 0, len(instances))
}

func TestStopUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	plugin, grp := mockedPluginGroup(ctrl)

	plugin.EXPECT().Validate(gomock.Any()).AnyTimes().Return(nil)

	require.NoError(t, grp.WatchGroup(minions))

	updated := group.Configuration{ID: id, Role: minions.Role, Properties: minionProperties(3, "data2")}

	partialUpdate := make(chan bool)

	gomock.InOrder(
		expectDescribe(plugin, id).Return([]instance.Description{
			createFake(minions, "a"),
			createFake(minions, "b"),
			createFake(minions, "c"),
		}, nil).MinTimes(1),
		plugin.EXPECT().Destroy(instance.ID("a")).Do(func(_ instance.ID) {
			partialUpdate <- true
		}).Return(nil),
	)

	go grp.UpdateGroup(updated)
	<-partialUpdate
	require.NoError(t, grp.StopUpdate(id))
	require.Error(t, grp.StopUpdate(id))
	require.NoError(t, grp.UnwatchGroup(id))
}

func TestSuperviseQuorum(t *testing.T) {
	// TODO(wfarner): This is not including IP addresses, so the quorum controller immediately destroys them.
	plugin := NewTestInstancePlugin(
		provisionTags(leaders),
		provisionTags(leaders),
		provisionTags(leaders),
	)
	grp := NewGroupPlugin(fakeInstancePluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond)

	require.NoError(t, grp.WatchGroup(leaders))

	updated := group.Configuration{ID: id, Role: leaders.Role, Properties: leaderProperties(managerIPs, "data2")}

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

	// TODO(wfarner): Validate IP addresses in created instances.
}
