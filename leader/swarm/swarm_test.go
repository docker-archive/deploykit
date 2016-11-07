package swarm

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/leader"
	docker_mock "github.com/docker/infrakit/mock/docker/docker/client"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestSwarmDetector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	nodeInfo := types.Info{
		Swarm: swarm.Info{
			NodeID: "node",
		},
	}
	node := swarm.Node{
		ManagerStatus: &swarm.ManagerStatus{
			Leader: true,
		},
	}

	mock := docker_mock.NewMockAPIClient(ctrl)
	mock.EXPECT().Info(ctx).Return(nodeInfo, nil).AnyTimes()
	mock.EXPECT().NodeInspectWithRaw(ctx, "node").Return(node, nil, nil).AnyTimes()

	detector := NewDetector(10*time.Millisecond, mock)

	events, err := detector.Start()
	require.NoError(t, err)
	require.NotNil(t, events)

	count := 10
	for event := range events {
		require.Equal(t, leader.StatusLeader, event.Status)
		count += -1
		if count == 0 {
			detector.Stop()
		}
	}
	// tests that we will stop cleanly here
}
