package swarm

import (
	"net/url"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/leader"
	docker_mock "github.com/docker/infrakit/pkg/mock/docker/docker/client"
	mock_client "github.com/docker/infrakit/pkg/mock/docker/docker/client"
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

	mock := docker_mock.NewMockAPIClientCloser(ctrl)
	mock.EXPECT().Info(ctx).Return(nodeInfo, nil).AnyTimes()
	mock.EXPECT().NodeInspectWithRaw(ctx, "node").Return(node, nil, nil).AnyTimes()

	detector := NewDetector(10*time.Millisecond, mock)

	events, err := detector.Start()
	require.NoError(t, err)
	require.NotNil(t, events)

	count := 10
	for event := range events {
		require.Equal(t, leader.Leader, event.Status)
		count += -1
		if count == 0 {
			detector.Stop()
		}
	}

	<-events // ensures clean stop
}

func TestSwarmStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_client.NewMockAPIClientCloser(ctrl)

	loc := "tcp://10.10.1.100:24864"
	u, err := url.Parse(loc)
	require.NoError(t, err)

	store := Store{client}

	swarmInfo := swarm.Swarm{}
	expectedSpec := swarm.Spec{
		Annotations: swarm.Annotations{
			Labels: map[string]string{SwarmLabel: u.String()},
		},
	}
	//Test first time save location (nil -> expectedSpec)

	client.EXPECT().SwarmUpdate(gomock.Any(), swarm.Version{Index: uint64(0)}, expectedSpec, swarm.UpdateFlags{RotateWorkerToken: false, RotateManagerToken: false, RotateManagerUnlockKey: false}).Return(nil)
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)
	err = store.UpdateLocation(u)
	require.NoError(t, err)

	//Test update Snapshot(unexpectedSpec -> expectedSpec)
	unexpectedSpec := swarm.Spec{
		Annotations: swarm.Annotations{
			Labels: map[string]string{SwarmLabel: "dummy"},
		},
	}
	swarmInfo.Spec = unexpectedSpec
	client.EXPECT().SwarmUpdate(gomock.Any(), swarm.Version{Index: uint64(0)}, expectedSpec, swarm.UpdateFlags{RotateWorkerToken: false, RotateManagerToken: false, RotateManagerUnlockKey: false}).Return(nil)
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)
	err = store.UpdateLocation(u)
	require.NoError(t, err)
	//Test Load Snapshot
	client.EXPECT().SwarmInspect(gomock.Any()).Return(swarmInfo, nil)

	uu, err := store.GetLocation()
	require.NoError(t, err)
	require.Equal(t, u.String(), uu.String())
}
