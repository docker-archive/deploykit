package group

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
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

func flavorPluginLookup(_ plugin_base.Name) (flavor.Plugin, error) {
	return &testFlavor{}, nil
}

func minionProperties(instances int, instanceData string, flavorInit string) *json.RawMessage {
	r := json.RawMessage(fmt.Sprintf(`{
	  "Allocation": {
	    "Size": %d
	  },
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
	          "Init": "%s"
	      }
          }
	}`, instances, instanceData, flavorInit))
	return &r
}

func leaderProperties(logicalIDs []instance.LogicalID, data string) *json.RawMessage {
	idsValue, err := json.Marshal(logicalIDs)
	if err != nil {
		panic(err)
	}

	r := json.RawMessage(fmt.Sprintf(`{
	  "Allocation": {
	    "LogicalIDs": %s
	  },
	  "Instance" : {
              "Plugin": "test",
	      "Properties": {
	          "OpaqueValue": "%s"
	      }
          },
	  "Flavor" : {
              "Plugin": "test",
	      "Properties": {
	         "Type": "leader"
	      }
          }
	}`, idsValue, data))
	return &r
}

func pluginLookup(pluginName string, plugin instance.Plugin) InstancePluginLookup {
	return func(key plugin_base.Name) (instance.Plugin, error) {
		if key.String() == pluginName {
			return plugin, nil
		}
		return nil, nil
	}
}

func TestInvalidGroupCalls(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	require.Error(t, grp.DestroyGroup(id))
	_, err := grp.DescribeGroup(id)
	require.Error(t, err)
	require.Error(t, grp.FreeGroup(id))
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
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	desc, err := grp.CommitGroup(minions, true)
	require.NoError(t, err)
	require.Equal(t, "Noop", desc)

	_, err = grp.CommitGroup(minions, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(minions.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, newFakeInstance(minions, nil).Tags, i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func awaitGroupConvergence(t *testing.T, grp group.Plugin) {
	for {
		desc, err := grp.DescribeGroup(id)
		require.NoError(t, err)
		if desc.Converged {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRollingUpdate(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(string(flavorProperties), "flavor2") {
				return flavor.Healthy, nil
			}

			// The update should be unaffected by an 'old' instance that is unhealthy.
			return flavor.Unhealthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup, 1*time.Millisecond, 0)
	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data2", "flavor2")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	awaitGroupConvergence(t, grp)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestRollAndAdjustScale(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data2", "flavor2")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(
		t,
		"Performing a rolling update on 3 instances, then adding 5 instances to increase the group size to 8",
		desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	awaitGroupConvergence(t, grp)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestScaleIncrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data", "init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Adding 5 instances to increase the group size to 8", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestScaleDecrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(1, "data", "init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Terminating 2 instances to reduce the group size to 1", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) <= 3)
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestFreeGroup(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	require.NoError(t, grp.FreeGroup(id))
}

func TestDestroyGroup(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

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
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(leaders, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: leaderProperties(leaderIDs, "data2")}

	time.Sleep(1 * time.Second)

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	awaitGroupConvergence(t, grp)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID))
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTags(updated), i.Tags)
	}

	// TODO(wfarner): Validate logical IDs in created instances.

	require.NoError(t, grp.FreeGroup(id))
}

func TestUpdateCompletes(t *testing.T) {
	// Tests that a completed update clears the 'update in progress state', allowing another update to commence.

	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, "data", "init")}
	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	updated = group.Spec{ID: id, Properties: minionProperties(5, "data", "init")}
	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, grp.FreeGroup(id))
}

func TestInstanceAndFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data2", "updated init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	awaitGroupConvergence(t, grp)

	for _, inst := range plugin.instancesCopy() {
		require.Equal(t, "updated init", inst.Init)

		properties := map[string]string{}
		err = json.Unmarshal(types.RawMessage(inst.Properties), &properties)
		require.NoError(t, err)
		require.Equal(t, "data2", properties["OpaqueValue"])
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data", "updated init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, grp.FreeGroup(id))
}

func TestFreeGroupWhileConverging(t *testing.T) {

	// Ensures that the group can be ignored while a commit is converging.

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)

	var once sync.Once
	healthChecksStarted := make(chan bool)
	defer close(healthChecksStarted)
	flavorPlugin := testFlavor{
		healthy: func(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(string(flavorProperties), "flavor2") {
				// sync.Once is used to prevent writing to healthChecksStarted more than one time,
				// causing the test to deadlock.
				once.Do(func() {
					healthChecksStarted <- true
				})
			}

			// Unknown health will stall the update indefinitely.
			return flavor.Unknown, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	awaitGroupConvergence(t, grp)

	// Since we expect only a single write to healthChecksStarted, it's important to use only one instance here.
	// This prevents flaky behavior where another health check is performed before StopUpdate() is called, leading
	// to a deadlock.
	updated := group.Spec{ID: id, Properties: minionProperties(3, "data", "flavor2")}

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	// Wait for the first health check to ensure the update has begun.
	<-healthChecksStarted

	require.NoError(t, grp.FreeGroup(id))
}

func TestUpdateFailsWhenInstanceIsUnhealthy(t *testing.T) {

	plugin := newTestInstancePlugin(
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
		newFakeInstance(minions, nil),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(string(flavorProperties), "bad update") {
				return flavor.Unhealthy, nil
			}
			return flavor.Healthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup, 1*time.Millisecond, 0)

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, "data", "bad update")}

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	awaitGroupConvergence(t, grp)

	// Only one instance should exist in the new configuration.
	badUpdateInstanaces := 0
	for _, inst := range plugin.instancesCopy() {
		if inst.Init == "bad update" {
			badUpdateInstanaces++
		}
	}

	require.Equal(t, 1, badUpdateInstanaces)
	require.NoError(t, grp.FreeGroup(id))
}

func TestNoSideEffectsFromPretendCommit(t *testing.T) {
	// Tests that internal state is not modified by a GroupCommit with Pretend=true.

	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup, 1*time.Millisecond, 0)

	desc, err := grp.CommitGroup(minions, true)
	require.NoError(t, err)
	require.Equal(t, "Managing 3 instances", desc)

	desc, err = grp.CommitGroup(minions, true)
	require.NoError(t, err)
	require.Equal(t, "Managing 3 instances", desc)

	err = grp.FreeGroup(id)
	require.Error(t, err)
	require.Equal(t, "Group 'testGroup' is not being watched", err.Error())

	err = grp.DestroyGroup(id)
	require.Error(t, err)
	require.Equal(t, "Group 'testGroup' is not being watched", err.Error())

	desc, err = grp.CommitGroup(minions, true)
	require.NoError(t, err)
	require.Equal(t, "Managing 3 instances", desc)
}
