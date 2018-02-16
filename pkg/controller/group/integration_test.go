package group

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

const (
	id         = group.ID("testGroup")
	pluginName = "test"
)

var (
	emptyUpdating = group_types.Updating{}

	minions = group.Spec{
		ID:         id,
		Properties: minionProperties(3, emptyUpdating, "data", "init"),
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

func minionProperties(instances uint, updating group_types.Updating, instanceData string, flavorInit string) *types.Any {
	updatingStr, err := json.Marshal(updating)
	if err != nil {
		panic(err)
	}

	return types.AnyString(fmt.Sprintf(`{
	  "Allocation": {
	    "Size": %d
	  },
	  "Updating": %s,
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
	}`, instances, updatingStr, instanceData, flavorInit))
}

func leaderProperties(logicalIDs []instance.LogicalID, data string) *types.Any {
	idsValue, err := json.Marshal(logicalIDs)
	if err != nil {
		panic(err)
	}

	return types.AnyString(fmt.Sprintf(`{
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
}

func pluginLookup(pluginName string, plugin instance.Plugin) InstancePluginLookup {
	return func(key plugin_base.Name) (instance.Plugin, error) {
		if key.String() == pluginName {
			return plugin, nil
		}
		return nil, nil
	}
}

func TestValidateNoGroupID(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	p, is := grp.(*gController)
	require.True(t, is)
	settings, err := p.validate(group.Spec{})
	require.Error(t, err)
	require.EqualError(t, err, "Group ID must not be blank")
	require.Equal(t, groupSettings{}, settings)
}

func TestValidateNoAllocations(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	p, is := grp.(*gController)
	require.True(t, is)
	settings, err := p.validate(group.Spec{ID: group.ID("id")})
	require.Error(t, err)
	require.EqualError(t, err, "Allocation must not be blank")
	require.Equal(t, groupSettings{}, settings)
}

func TestValidateMultipleAllocations(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	p, is := grp.(*gController)
	require.True(t, is)
	spec := group_types.Spec{
		Allocation: group.AllocationMethod{
			Size:       1,
			LogicalIDs: []instance.LogicalID{instance.LogicalID("id1")},
		},
	}
	props, err := types.AnyValue(spec)
	require.NoError(t, err)
	settings, err := p.validate(group.Spec{
		ID:         group.ID("id"),
		Properties: props,
	})
	require.Error(t, err)
	require.EqualError(t, err, "Only one Allocation method may be used")
	require.Equal(t, groupSettings{}, settings)
}

func TestValidateMultipleUpdating(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	p, is := grp.(*gController)
	require.True(t, is)
	spec := group_types.Spec{
		Allocation: group.AllocationMethod{
			Size: 1,
		},
		Updating: group_types.Updating{
			Count:    1,
			Duration: types.MustParseDuration("1s"),
		},
	}
	props, err := types.AnyValue(spec)
	require.NoError(t, err)
	settings, err := p.validate(group.Spec{
		ID:         group.ID("id"),
		Properties: props,
	})
	require.Error(t, err)
	require.EqualError(t, err, "Only one Updating method may be used")
	require.Equal(t, groupSettings{}, settings)
}

func TestInvalidGroupCalls(t *testing.T) {
	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	require.Error(t, grp.DestroyGroup(id))
	_, err := grp.DescribeGroup(id)
	require.Error(t, err)
	require.Error(t, grp.FreeGroup(id))
}

// memberTags returns the tags with the group ID
func memberTags(id group.ID) map[string]string {
	return map[string]string{group.GroupTag: string(id)}
}

// provisionTagsDefault returns default tag values
func provisionTagsDefault(config group.Spec, logicalID *instance.LogicalID) map[string]string {
	return provisionTags(config, logicalID, map[string]string{})
}

// provisionTagsDestroyErr returns default tag values with the unique tag that reflects that
// the instance should error out on destroy
func provisionTagsDestroyErr(config group.Spec, logicalID *instance.LogicalID) map[string]string {
	return provisionTags(config, logicalID, map[string]string{"DestroyError": "true"})
}

// provisionTags returns tag values with the additional tags inserted
func provisionTags(config group.Spec, logicalID *instance.LogicalID, additional map[string]string) map[string]string {
	tags := memberTags(config.ID)
	tags[group.ConfigSHATag] = group_types.MustParse(group_types.ParseProperties(config)).InstanceHash()

	if logicalID != nil {
		tags[instance.LogicalIDTag] = string(*logicalID)
	}
	for k, v := range additional {
		tags[k] = v
	}
	return tags
}

// newFakeInstanceDefault returns an instance.Spec with the default tags
func newFakeInstanceDefault(config group.Spec, logicalID *instance.LogicalID) instance.Spec {
	return newFakeInstance(config, logicalID, provisionTagsDefault(config, logicalID))
}

// Creates an instance.Spec with a tag that denotes that the instance should return
// an error when Destroyed
func newFakeDestroyErrInstance(config group.Spec, logicalID *instance.LogicalID) instance.Spec {
	return newFakeInstance(config, logicalID, provisionTagsDestroyErr(config, logicalID))
}

// newFakeInstanceDefault returns an instance.Spec with the default tags
func newFakeInstance(config group.Spec, logicalID *instance.LogicalID, provisionTags map[string]string) instance.Spec {
	return instance.Spec{
		LogicalID: logicalID,
		Tags:      provisionTags,
	}
}

func TestNoopUpdate(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	desc, err := grp.CommitGroup(minions, true)
	require.NoError(t, err)
	require.Equal(t, "Noop", desc)

	_, err = grp.CommitGroup(minions, false)
	require.NoError(t, err)

	// Nothing drained or destroyed
	require.Empty(t, plugin.destroyed)
	require.Empty(t, flavorPlugin.drained)

	instances, err := plugin.DescribeInstances(memberTags(minions.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, newFakeInstanceDefault(minions, nil).Tags, i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))

}

func awaitGroupConvergence(t *testing.T, grp group.Plugin) error {
	start := time.Now()
	for {
		desc, err := grp.DescribeGroup(id)
		require.NoError(t, err)
		if desc.Converged {
			return nil
		}
		if time.Now().Sub(start) >= (time.Second * 2) {
			return fmt.Errorf("Has not converged in 2s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRollingUpdate(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(flavorProperties.String(), "flavor2") {
				return flavor.Healthy, nil
			}

			// The update should be unaffected by an 'old' instance that is unhealthy.
			return flavor.Unhealthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data2", "flavor2")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestRollingUpdateUpdatingCount(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	// Healthy should be invoked on each instance at least 2 times
	mutex := sync.Mutex{}
	healthyExecs := map[instance.ID]int{}

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			mutex.Lock()
			defer mutex.Unlock()
			if strings.Contains(flavorProperties.String(), "flavor2") {
				if data, has := healthyExecs[inst.ID]; has {
					healthyExecs[inst.ID] = data + 1
					// If we have 3 instances then return Unhealthy after the first
					// Healthy, this will cause the count to be reset
					if len(healthyExecs) == 3 && data == 1 {
						return flavor.Unhealthy, nil
					}
				} else {
					healthyExecs[inst.ID] = 1
				}
				return flavor.Healthy, nil
			}

			// The update should be unaffected by an 'old' instance that is unhealthy.
			return flavor.Unhealthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(100 * time.Millisecond),
		})
	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	// Update and require 2 healthy counts per node
	updated := group.Spec{
		ID:         id,
		Properties: minionProperties(3, group_types.Updating{Count: 2}, "data2", "flavor2"),
	}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}
	// Since the rolling update is sequential, the first health is invoked 6 times,
	// the second 4 times, and the last only 2 times. However, since the last one
	// returned Unhealthy on the 2nd query then all are queried again so the
	// expected counts are 8, 6, 4
	require.Len(t, healthyExecs, 3)
	keys := []string{}
	for _, k := range instances {
		keys = append(keys, string(k.ID))
	}
	sort.Strings(keys)
	sorted := []instance.Description{}
	for _, k := range keys {
		for _, i := range instances {
			if string(i.ID) == k {
				sorted = append(sorted, i)
				break
			}
		}
	}
	for index, inst := range sorted {
		data, has := healthyExecs[inst.ID]
		require.True(t, has, fmt.Sprintf("Missing instance ID: %s", inst.ID))
		expectedCount := []int{8, 6, 4}[index]
		require.Equal(t, expectedCount, data,
			fmt.Sprintf("Data for ID %s is not %d: %v", inst.ID, expectedCount, healthyExecs))
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestRollingUpdateUpdatingDuration(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	// Healthy should be invoked on each instance at least 2 times
	mutex := sync.Mutex{}
	healthyExecs := map[instance.ID]int{}

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			mutex.Lock()
			defer mutex.Unlock()
			if strings.Contains(flavorProperties.String(), "flavor2") {
				if data, has := healthyExecs[inst.ID]; has {
					healthyExecs[inst.ID] = data + 1
				} else {
					healthyExecs[inst.ID] = 1
				}
				return flavor.Healthy, nil
			}

			// The update should be unaffected by an 'old' instance that is unhealthy.
			return flavor.Unhealthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(2 * time.Millisecond),
		})
	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	// Update and require a 20ms duration (plugin polls every 2ms)
	updated := group.Spec{
		ID: id,
		Properties: minionProperties(3,
			group_types.Updating{Duration: types.FromDuration(20 * time.Millisecond)},
			"data2",
			"flavor2"),
	}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}
	// Each instance should be checked for health at least 2 times since the
	// duration is 10x the poll interval
	for _, inst := range instances {
		data, has := healthyExecs[inst.ID]
		require.True(t, has, fmt.Sprintf("Missing instance ID: %s", inst.ID))
		require.True(t,
			data >= 2,
			fmt.Sprintf("Data for ID %s is less than 2: %v", inst.ID, healthyExecs))
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestRollingUpdateDestroyError(t *testing.T) {
	// The 2nd instance will error out on Destroy, causing the 3rd instance to not be updated.
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeDestroyErrInstance(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(flavorProperties.String(), "flavor2") {
				return flavor.Healthy, nil
			}

			// The update should be unaffected by an 'old' instance that is unhealthy.
			return flavor.Unhealthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})
	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data2", "flavor2")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Only the first instance was successfully destroyed
	require.Len(t, plugin.destroyed, 1)
	// And the first 2 were drained
	require.Len(t, flavorPlugin.drained, 2)
	require.Equal(t, instance.ID(fmt.Sprintf("%s-1", plugin.idPrefix)), flavorPlugin.drained[0].ID)
	require.Equal(t, instance.ID(fmt.Sprintf("%s-2", plugin.idPrefix)), flavorPlugin.drained[1].ID)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	// 1st instance was destroyed and provisioned (given a new ID)
	instID := instance.ID(fmt.Sprintf("%s-4", plugin.idPrefix))
	inst1, has := plugin.instances[instID]
	require.True(t, has, fmt.Sprintf("Missing instance ID %s", instID))
	require.Equal(t, provisionTagsDefault(updated, nil), inst1.Tags)
	// 2nd instance failed to destroy which caused the 3rd instance to not be processed
	for _, idSuffix := range []int{2, 3} {
		instID := instance.ID(fmt.Sprintf("%s-%d", plugin.idPrefix, idSuffix))
		inst, has := plugin.instances[instID]
		require.True(t, has, fmt.Sprintf("Missing instance ID %s", instID))
		if idSuffix == 2 {
			require.Equal(t, provisionTagsDestroyErr(minions, nil), inst.Tags)
		} else {
			require.Equal(t, provisionTagsDefault(minions, nil), inst.Tags)
		}
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestLeaderSelfRollingUpdate(t *testing.T) {

	// This is the case where the controller coordinating the rolling update
	// is part of the group that is being updated
	self := &leaderIDs[0]

	// leader self rolling update should destroy itself (last)
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(leaders, &leaderIDs[0]),
		newFakeInstanceDefault(leaders, &leaderIDs[1]),
		newFakeInstanceDefault(leaders, &leaderIDs[2]),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			return flavor.Healthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
			Self:         self,
		})
	_, err := grp.CommitGroup(leaders, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(leaders.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))

	updated := group.Spec{ID: id, Properties: leaderProperties(leaderIDs, "data2")}

	desc, err := grp.CommitGroup(updated, true) // pretend only
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and self not destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 2)

	instances, err = plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))

	// Non-self instances are updated
	selfCount, nonSelfCount := 0, 0
	for _, inst := range instances {
		if self == inst.LogicalID {
			require.Equal(t, provisionTagsDefault(leaders, inst.LogicalID), inst.Tags)
			selfCount++
		} else {
			require.Equal(t, provisionTagsDefault(updated, inst.LogicalID), inst.Tags)
			nonSelfCount++
		}
	}
	require.Equal(t, 1, selfCount)
	require.Equal(t, 2, nonSelfCount)

	// All instances are drained
	require.Len(t, flavorPlugin.drained, 3)
	// Non-self instances destroyed
	require.Len(t, plugin.destroyed, 2)
	for _, destroyed := range plugin.destroyed {
		require.NotEqual(t, self, destroyed.LogicalID)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestExternalManagedRollingUpdate(t *testing.T) {

	// This is the case where the controller coordinating the rolling update
	// is a node that's not part of the leader group
	other := instance.LogicalID("other")
	self := &other

	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(leaders, &leaderIDs[0]),
		newFakeInstanceDefault(leaders, &leaderIDs[1]),
		newFakeInstanceDefault(leaders, &leaderIDs[2]),
	)

	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			return flavor.Healthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
			Self:         self,
		})

	_, err := grp.CommitGroup(leaders, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(leaders.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))

	updated := group.Spec{ID: id, Properties: leaderProperties(leaderIDs, "data2")}

	desc, err := grp.CommitGroup(updated, true) // pretend only
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err = plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))

	for i := 0; i < len(instances); i++ {

		// since this is an external manager, not part of the group being updated,
		require.NotEqual(t, *instances[i].LogicalID, *self)

		// we require every node to have been updated
		require.Equal(t, provisionTagsDefault(updated, instances[i].LogicalID), instances[i].Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestRollingUpdateNoDrain(t *testing.T) {
	spec := group.Spec{
		ID: id,
		Properties: minionProperties(3,
			group_types.Updating{SkipBeforeInstanceDestroy: &group_types.SkipBeforeInstanceDestroyDrain},
			"data",
			"init"),
	}
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(spec, nil),
		newFakeInstanceDefault(spec, nil),
		newFakeInstanceDefault(spec, nil),
	)
	flavorPlugin := testFlavor{
		drain: func(flavorProperties *types.Any, inst instance.Description) error {
			require.Fail(t, "Drain should not be invoked")
			return fmt.Errorf("Should not be invoked")
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(spec, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(spec.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))

	updated := group.Spec{
		ID: id,
		Properties: minionProperties(3,
			group_types.Updating{SkipBeforeInstanceDestroy: &group_types.SkipBeforeInstanceDestroyDrain},
			"data2",
			"init"),
	}

	desc, err := grp.CommitGroup(updated, true) // pretend only
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	desc, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Nothing drained and everything destroyed
	require.Len(t, flavorPlugin.drained, 0)
	require.Len(t, plugin.destroyed, 3)

	instances, err = plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
}

func TestRollAndAdjustScale(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, emptyUpdating, "data2", "flavor2")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(
		t,
		"Performing a rolling update on 3 instances, then adding 5 instances to increase the group size to 8",
		desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestScaleIncrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, emptyUpdating, "data", "init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Adding 5 instances to increase the group size to 8", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) >= 3)
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestScaleDecrease(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(1, emptyUpdating, "data", "init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Terminating 2 instances to reduce the group size to 1", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	// TODO(wfarner): The updater currently exits as soon as the scaler is adjusted, before action has been
	// taken.  This means the number of instances cannot be precisely checked here as the scaler has not necessarily
	// quiesced.
	require.True(t, len(instances) <= 3)
	for _, i := range instances {
		require.Equal(t, provisionTagsDefault(updated, nil), i.Tags)
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestFreeGroup(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	require.NoError(t, grp.FreeGroup(id))
}

func TestDestroyGroup(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	require.NoError(t, grp.DestroyGroup(minions.ID))

	instances, err := plugin.DescribeInstances(memberTags(minions.ID), false)
	require.NoError(t, err)
	require.Equal(t, 0, len(instances))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)
}

func TestDestroyGroupSelfLast(t *testing.T) {
	self := &leaderIDs[1]

	// leader self should destroy itself last
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(leaders, &leaderIDs[0]),
		newFakeInstanceDefault(leaders, &leaderIDs[1]),
		newFakeInstanceDefault(leaders, &leaderIDs[2]),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
			Self:         self,
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	require.NoError(t, grp.DestroyGroup(minions.ID))

	// All instance should have been removed (since this is a group destroy)
	instances, err := plugin.DescribeInstances(memberTags(minions.ID), false)
	require.NoError(t, err)
	require.Equal(t, 0, len(instances))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	// Self should have been removed last
	require.Equal(t, *self, *plugin.destroyed[2].LogicalID)
}

func TestSuperviseQuorum(t *testing.T) {
	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(leaders, &leaderIDs[0]),
		newFakeInstanceDefault(leaders, &leaderIDs[1]),
		newFakeInstanceDefault(leaders, &leaderIDs[2]),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(leaders, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: leaderProperties(leaderIDs, "data2")}

	time.Sleep(1 * time.Second)

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	instances, err := plugin.DescribeInstances(memberTags(updated.ID), false)
	require.NoError(t, err)
	require.Equal(t, 3, len(instances))
	for _, i := range instances {

		expectTags := provisionTagsDefault(updated, i.LogicalID)
		if i.LogicalID != nil {
			// expect to also have a label with logical ID
			expectTags[instance.LogicalIDTag] = string(*i.LogicalID)
		}
		require.Equal(t, expectTags, i.Tags)
	}

	// TODO(wfarner): Validate logical IDs in created instances.

	require.NoError(t, grp.FreeGroup(id))
}

func TestUpdateCompletes(t *testing.T) {
	// Tests that a completed update clears the 'update in progress state', allowing another update to commence.

	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(8, emptyUpdating, "data", "init")}
	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	updated = group.Spec{ID: id, Properties: minionProperties(5, emptyUpdating, "data", "init")}
	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, grp.FreeGroup(id))
}

func TestInstanceAndFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	flavorPlugin := testFlavor{}
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin),
		func(_ plugin_base.Name) (flavor.Plugin, error) { return &flavorPlugin, nil },
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data2", "updated init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Everything drained and destroyed
	require.Len(t, flavorPlugin.drained, 3)
	require.Len(t, plugin.destroyed, 3)

	for _, inst := range plugin.instancesCopy() {
		require.Equal(t, "updated init", inst.Init)

		properties := map[string]string{}
		err = types.AnyBytes([]byte(*inst.Properties)).Decode(&properties)
		require.NoError(t, err)
		require.Equal(t, "data2", properties["OpaqueValue"])
	}

	require.NoError(t, grp.FreeGroup(id))
}

func TestFlavorChange(t *testing.T) {
	// Tests that a change to the flavor configuration triggers an update.

	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data", "updated init")}

	desc, err := grp.CommitGroup(updated, true)
	require.NoError(t, err)
	require.Equal(t, "Performing a rolling update on 3 instances", desc)

	require.NoError(t, grp.FreeGroup(id))
}

func TestFreeGroupWhileConverging(t *testing.T) {

	// Ensures that the group can be ignored while a commit is converging.

	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	var once sync.Once
	healthChecksStarted := make(chan bool)
	defer close(healthChecksStarted)
	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			if strings.Contains(flavorProperties.String(), "flavor2") {
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

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// Since we expect only a single write to healthChecksStarted, it's important to use only one instance here.
	// This prevents flaky behavior where another health check is performed before StopUpdate() is called, leading
	// to a deadlock.
	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data", "flavor2")}

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	// Wait for the first health check to ensure the update has begun.
	<-healthChecksStarted

	require.NoError(t, grp.FreeGroup(id))
}

func TestUpdateFailsWhenInstanceIsUnhealthy(t *testing.T) {
	// An Unhealthy instance does not stop the polling, it will continue until
	// the instance is healthy

	plugin := newTestInstancePlugin(
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
		newFakeInstanceDefault(minions, nil),
	)

	unhealthyCount := 0
	flavorPlugin := testFlavor{
		healthy: func(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {
			if unhealthyCount == 0 && strings.Contains(flavorProperties.String(), "bad update") {
				unhealthyCount++
				return flavor.Unhealthy, nil
			}
			return flavor.Healthy, nil
		},
	}
	flavorLookup := func(_ plugin_base.Name) (flavor.Plugin, error) {
		return &flavorPlugin, nil
	}

	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

	_, err := grp.CommitGroup(minions, false)
	require.NoError(t, err)

	updated := group.Spec{ID: id, Properties: minionProperties(3, emptyUpdating, "data", "bad update")}

	_, err = grp.CommitGroup(updated, false)
	require.NoError(t, err)

	require.NoError(t, awaitGroupConvergence(t, grp))

	// All instances should have been updated.
	badUpdateInstanaces := 0
	for _, inst := range plugin.instancesCopy() {
		if inst.Init == "bad update" {
			badUpdateInstanaces++
		}
	}

	require.Equal(t, 3, badUpdateInstanaces)
	require.NoError(t, grp.FreeGroup(id))
}

func TestNoSideEffectsFromPretendCommit(t *testing.T) {
	// Tests that internal state is not modified by a GroupCommit with Pretend=true.

	plugin := newTestInstancePlugin()
	grp := NewGroupPlugin(pluginLookup(pluginName, plugin), flavorPluginLookup,
		group_types.Options{
			PollInterval: types.FromDuration(1 * time.Millisecond),
		})

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
