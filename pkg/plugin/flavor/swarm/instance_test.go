package swarm

import (
	"fmt"
	"testing"

	docker_types "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	mock_client "github.com/docker/infrakit/pkg/mock/docker/docker/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/docker"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestSwarmInstDescribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	client.EXPECT().NodeList(gomock.Any(), gomock.Any()).Return([]swarm.Node{}, fmt.Errorf("custom-error")).Times(1)
	client.EXPECT().Close().AnyTimes()

	insts, err := p.DescribeInstances(map[string]string{}, true)
	require.Error(t, err)
	require.EqualError(t, err, "custom-error")
	require.Equal(t, []instance.Description{}, insts)
}

func TestSwarmInstDescribeNoNodes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	client.EXPECT().NodeList(gomock.Any(), gomock.Any()).Return([]swarm.Node{}, nil).Times(1)
	client.EXPECT().Close().AnyTimes()

	insts, err := p.DescribeInstances(map[string]string{}, true)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{}, insts)
}

func TestSwarmInstDescribeNodesWithProps(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	// Node1 has a link label
	node1 := swarm.Node{
		ID: "node-id1",
		Description: swarm.NodeDescription{
			Hostname: "node-host1",
			Engine: swarm.EngineDescription{
				Labels: map[string]string{
					"node1-l1":      "val1-1",
					"node1-l2":      "val1-2",
					types.LinkLabel: "link-label-1",
				},
			},
		},
	}
	// Node2 is missing a link label
	node2 := swarm.Node{
		ID: "node-id2",
		Description: swarm.NodeDescription{
			Hostname: "node-host2",
			Engine: swarm.EngineDescription{
				Labels: map[string]string{
					"node2-l1": "val2-1",
					"node2-l2": "val2-2",
				},
			},
		},
	}
	expectedProps1, _ := types.AnyValue(node1)
	expectedProps2, _ := types.AnyValue(node2)

	nodes := []swarm.Node{node1, node2}
	client.EXPECT().NodeList(gomock.Any(), gomock.Any()).Return(nodes, nil).Times(1)
	client.EXPECT().Close().AnyTimes()

	insts, err := p.DescribeInstances(map[string]string{}, true)
	require.NoError(t, err)
	expectedLogicalID1 := instance.LogicalID("link-label-1")
	require.Equal(t,
		[]instance.Description{
			{
				ID:        instance.ID("node-id1"),
				LogicalID: &expectedLogicalID1,
				Tags: map[string]string{
					"node1-l1":      "val1-1",
					"node1-l2":      "val1-2",
					types.LinkLabel: "link-label-1",
					"name":          "node-host1",
				},
				Properties: expectedProps1,
			},
			{
				ID:        instance.ID("node-id2"),
				LogicalID: nil,
				Tags: map[string]string{
					"node2-l1": "val2-1",
					"node2-l2": "val2-2",
					"name":     "node-host2",
				},
				Properties: expectedProps2,
			},
		},
		insts)
}

func TestSwarmInstDescribeNodesWithoutProps(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	// Node1 has a link label
	node1 := swarm.Node{
		ID: "node-id1",
		Description: swarm.NodeDescription{
			Hostname: "node-host1",
			Engine: swarm.EngineDescription{
				Labels: map[string]string{
					"node1-l1":      "val1-1",
					"node1-l2":      "val1-2",
					types.LinkLabel: "link-label-1",
				},
			},
		},
	}
	// Node2 is missing a link label
	node2 := swarm.Node{
		ID: "node-id2",
		Description: swarm.NodeDescription{
			Hostname: "node-host2",
			Engine: swarm.EngineDescription{
				Labels: map[string]string{
					"node2-l1": "val2-1",
					"node2-l2": "val2-2",
				},
			},
		},
	}

	nodes := []swarm.Node{node1, node2}
	client.EXPECT().NodeList(gomock.Any(), gomock.Any()).Return(nodes, nil).Times(1)
	client.EXPECT().Close().AnyTimes()

	insts, err := p.DescribeInstances(map[string]string{}, false)
	require.NoError(t, err)
	expectedLogicalID1 := instance.LogicalID("link-label-1")
	require.Equal(t,
		[]instance.Description{
			{
				ID:        instance.ID("node-id1"),
				LogicalID: &expectedLogicalID1,
				Tags: map[string]string{
					"node1-l1":      "val1-1",
					"node1-l2":      "val1-2",
					types.LinkLabel: "link-label-1",
					"name":          "node-host1",
				},
				Properties: nil,
			},
			{
				ID:        instance.ID("node-id2"),
				LogicalID: nil,
				Tags: map[string]string{
					"node2-l1": "val2-1",
					"node2-l2": "val2-2",
					"name":     "node-host2",
				},
				Properties: nil,
			},
		},
		insts)
}

func TestSwarmInstDestroyNoNodes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	nodeID := "abc1234"
	client.EXPECT().Close().AnyTimes()
	expectedErr := fmt.Errorf("no-node-error")
	client.EXPECT().NodeInspectWithRaw(context.Background(), nodeID).Return(swarm.Node{}, nil, expectedErr).Times(1)

	err := p.Destroy(instance.ID(nodeID), instance.Termination)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func TestSwarmInstDestroyManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	nodeID := "abc1234"
	nodeVersion := uint64(1234)
	node := swarm.Node{
		ID: nodeID,
		Spec: swarm.NodeSpec{
			Role: swarm.NodeRoleManager,
		},
		Meta: swarm.Meta{
			Version: swarm.Version{
				Index: nodeVersion,
			},
		},
	}
	nodeSpecUpdate := swarm.NodeSpec{
		Role: swarm.NodeRoleWorker,
	}
	client.EXPECT().Close().AnyTimes()
	client.EXPECT().NodeInspectWithRaw(context.Background(), nodeID).Return(node, nil, nil).Times(1)
	client.EXPECT().NodeUpdate(
		context.Background(),
		nodeID,
		swarm.Version{Index: nodeVersion},
		nodeSpecUpdate).Return(nil).Times(1)
	client.EXPECT().NodeRemove(
		context.Background(),
		nodeID,
		docker_types.NodeRemoveOptions{Force: true}).Return(nil).Times(1)

	err := p.Destroy(instance.ID(nodeID), instance.Termination)
	require.NoError(t, err)
}

func TestSwarmInstDestroyWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	nodeID := "abc1234"
	nodeVersion := uint64(1234)
	node := swarm.Node{
		ID: nodeID,
		Spec: swarm.NodeSpec{
			Role: swarm.NodeRoleWorker,
		},
		Meta: swarm.Meta{
			Version: swarm.Version{
				Index: nodeVersion,
			},
		},
	}
	client.EXPECT().Close().AnyTimes()
	client.EXPECT().NodeInspectWithRaw(context.Background(), nodeID).Return(node, nil, nil).Times(1)
	client.EXPECT().NodeRemove(
		context.Background(),
		nodeID,
		docker_types.NodeRemoveOptions{Force: true}).Return(nil).Times(1)

	err := p.Destroy(instance.ID(nodeID), instance.Termination)
	require.NoError(t, err)
}

func TestSwarmInstValidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	client.EXPECT().Close().AnyTimes()

	err := p.Validate(nil)
	require.Error(t, err)
	require.EqualError(t, err, "Validate not supported for swarm instance")
}

func TestSwarmInstProvision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	client.EXPECT().Close().AnyTimes()

	id, err := p.Provision(instance.Spec{})
	require.Error(t, err)
	require.EqualError(t, err, "Provision not supported for swarm instance")
	require.Nil(t, id)
}

func TestSwarmInstLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_client.NewMockAPIClientCloser(ctrl)
	p := NewInstancePlugin(func(Spec) (docker.APIClientCloser, error) {
		return client, nil
	}, docker.ConnectInfo{})

	client.EXPECT().Close().AnyTimes()

	err := p.Label(instance.ID(""), map[string]string{})
	require.Error(t, err)
	require.EqualError(t, err, "Label not supported for swarm instance")
}
