package swarm

import (
	"errors"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/libmachete/mock/docker/engine-api/client"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
	"time"
)

func TestRunWhenLeading(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dockerClient := client.NewMockAPIClient(ctrl)

	callbacks := []string{}

	leadingStart := func() {
		callbacks = append(callbacks, "start")
	}

	leadingEnd := func() {
		callbacks = append(callbacks, "end")
	}

	dockerClient.EXPECT().Info(gomock.Any()).Return(types.Info{Swarm: swarm.Info{NodeID: "a"}}, nil).AnyTimes()

	leading := swarm.Node{ManagerStatus: &swarm.ManagerStatus{Leader: true}}
	notLeading := swarm.Node{ManagerStatus: &swarm.ManagerStatus{Leader: false}}

	gomock.InOrder(
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").
			Return(swarm.Node{}, nil, errors.New("This node is not a Swarm manager")),
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").Return(notLeading, nil, nil),
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").Return(notLeading, nil, nil),
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").Return(leading, nil, nil),
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").Return(leading, nil, nil),
		dockerClient.EXPECT().NodeInspectWithRaw(gomock.Any(), "a").Return(notLeading, nil, nil),
	)

	done := make(chan bool)
	go func() {
		err := RunWhenLeading(context.Background(), dockerClient, 5*time.Millisecond, leadingStart, leadingEnd)
		require.NoError(t, err)
		done <- true
	}()

	<-done
	require.Equal(t, []string{"start", "end"}, callbacks)
}
