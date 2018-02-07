package swarm

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

// FakeInstancePlugin is the fake swarm instance plugin used for testing
type FakeInstancePlugin struct {
	DescribeInstancesStub func(labels map[string]string, properties bool) ([]instance.Description, error)
}

// DescribeInstances .
func (fake FakeInstancePlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	if len(labels) > 0 {
		return nil, fmt.Errorf("No labels should be supplied")
	}
	if !properties {
		return nil, fmt.Errorf("properties should be true")
	}
	return fake.DescribeInstancesStub(labels, properties)
}

// Validate .
func (fake FakeInstancePlugin) Validate(req *types.Any) error {
	return fmt.Errorf("Validate not implemented on FakeInstancePlugin")
}

// Provision .
func (fake FakeInstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	return nil, fmt.Errorf("Provision not implemented on FakeInstancePlugin")
}

// Label .
func (fake FakeInstancePlugin) Label(instance instance.ID, labels map[string]string) error {
	return fmt.Errorf("Label not implemented on FakeInstancePlugin")
}

// Destroy .
func (fake FakeInstancePlugin) Destroy(instance instance.ID, context instance.Context) error {
	return fmt.Errorf("Destroy not implemented on FakeInstancePlugin")
}

func TestSwarmGroupDescribeError(t *testing.T) {
	fake := FakeInstancePlugin{
		DescribeInstancesStub: func(labels map[string]string, properties bool) ([]instance.Description, error) {
			return nil, fmt.Errorf("custom-error")
		},
	}
	p := NewGroupPlugin(fake)
	result, err := p.DescribeGroup(group.ID(""))
	require.Error(t, err)
	require.EqualError(t, err, "custom-error")
	require.Equal(t, group.Description{}, result)
}

func TestSwarmGroupDescribe(t *testing.T) {

	// Node1 is ready with a logical ID
	node1 := swarm.Node{
		ID: "node-id1",
		Description: swarm.NodeDescription{
			Hostname: "node-host1",
			Engine: swarm.EngineDescription{
				Labels: map[string]string{
					"node1-l1":      "val1-1",
					types.LinkLabel: "link-label-1",
				},
			},
		},
		Status: swarm.NodeStatus{
			State: swarm.NodeStateReady,
		},
	}
	encodedNode1, err := types.AnyValue(node1)
	require.NoError(t, err)
	logicalID1 := instance.LogicalID("link-label-1")
	inst1 := instance.Description{
		LogicalID:  &logicalID1,
		Properties: encodedNode1,
	}
	// Node 2 is ready without a logical ID
	node2 := swarm.Node{
		ID: "node-id2",
		Description: swarm.NodeDescription{
			Hostname: "node-host2",
		},
		Status: swarm.NodeStatus{
			State: swarm.NodeStateReady,
		},
	}
	encodedNode2, err := types.AnyValue(node2)
	require.NoError(t, err)
	inst2 := instance.Description{
		LogicalID:  nil,
		Properties: encodedNode2,
	}
	// Node3 is not ready with a logical ID
	node3 := swarm.Node{
		ID: "node-id3",
		Description: swarm.NodeDescription{
			Hostname: "node-host3",
		},
		Status: swarm.NodeStatus{
			State: swarm.NodeStateDown,
		},
	}
	encodedNode3, err := types.AnyValue(node3)
	require.NoError(t, err)
	logicalID3 := instance.LogicalID("link-label-3")
	inst3 := instance.Description{
		LogicalID:  &logicalID3,
		Properties: encodedNode3,
	}
	// Node 4 is empty with a logical ID
	node4 := swarm.Node{}
	encodedNode4, err := types.AnyValue(node4)
	require.NoError(t, err)
	logicalID4 := instance.LogicalID("link-label-4")
	inst4 := instance.Description{
		LogicalID:  &logicalID4,
		Properties: encodedNode4,
	}

	fake := FakeInstancePlugin{
		DescribeInstancesStub: func(labels map[string]string, properties bool) ([]instance.Description, error) {
			return []instance.Description{inst1, inst2, inst3, inst4}, nil
		},
	}
	p := NewGroupPlugin(fake)
	result, err := p.DescribeGroup(group.ID(""))
	require.NoError(t, err)
	require.Equal(t,
		group.Description{
			Instances: []instance.Description{
				{
					LogicalID:  &logicalID1,
					Properties: encodedNode1,
				},
			},
			Converged: false,
		}, result)
}

func TestSwarmGroupSize(t *testing.T) {
	fake := FakeInstancePlugin{
		DescribeInstancesStub: func(labels map[string]string, properties bool) ([]instance.Description, error) {
			return []instance.Description{}, nil
		},
	}
	p := NewGroupPlugin(fake)
	size, err := p.Size(group.ID(""))
	require.NoError(t, err)
	require.Equal(t, 0, size)
}

func TestSwarmGroupCommitGroup(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	id, err := p.CommitGroup(group.Spec{}, true)
	require.Error(t, err)
	require.EqualError(t, err, "CommitGroup not supported for swarm group")
	require.Equal(t, "", id)
}

func TestSwarmGroupFreeGroup(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	err := p.FreeGroup(group.ID(""))
	require.Error(t, err)
	require.EqualError(t, err, "FreeGroup not supported for swarm group")
}

func TestSwarmGroupInspectGroups(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	spec, err := p.InspectGroups()
	require.Error(t, err)
	require.EqualError(t, err, "InspectGroups not supported for swarm group")
	require.Equal(t, []group.Spec{}, spec)
}

func TestSwarmGroupDestroyGroup(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	err := p.DestroyGroup(group.ID(""))
	require.Error(t, err)
	require.EqualError(t, err, "DestroyGroup not supported for swarm group")
}

func TestSwarmGroupDestroyInstances(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	err := p.DestroyInstances(group.ID(""), []instance.ID{})
	require.Error(t, err)
	require.EqualError(t, err, "DestroyInstances not supported for swarm group")
}

func TestSwarmGroupSetSize(t *testing.T) {
	fake := FakeInstancePlugin{}
	p := NewGroupPlugin(fake)
	err := p.SetSize(group.ID(""), 0)
	require.Error(t, err)
	require.EqualError(t, err, "SetSize not supported for swarm group")
}
