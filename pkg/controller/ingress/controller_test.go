package ingress

import (
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type fakeLeadership <-chan bool

func (l fakeLeadership) IsLeader() (bool, error) {
	return <-l, nil
}

func TestControllerStartStop(t *testing.T) {

	ticker := make(chan time.Time, 1)
	leader := make(chan bool, 1)

	doneWork := make(chan interface{})

	controller := &Controller{
		leader:           fakeLeadership(leader),
		ticker:           ticker,
		healthChecks:     func() (map[Vhost][]HealthCheck, error) { return nil, nil },
		groups:           func() (map[Vhost][]group.ID, error) { return nil, nil },
		groupPluginNames: func() map[Vhost][]plugin.Name { return nil },
		l4s:              func() (map[Vhost]loadbalancer.L4, error) { return nil, nil },

		routes: func() (map[Vhost][]loadbalancer.Route, error) {
			// if this function is called then we know we've done work in the state transition
			// from syncing to waiting
			close(doneWork)
			return nil, nil
		},
	}

	spec := types.Spec{
		Kind:       "ingress-controller",
		SpiVersion: "0.1",
		Metadata: types.Metadata{
			Name: "ingress-controller",
		},
		Properties: types.AnyValueMust(Properties{
			{
				Vhost:      Vhost("default"),
				ResourceID: "elb-1",
			},
		}),
	}
	err := controller.init(spec)
	require.NoError(t, err)

	controller.start()

	t.Log("verify initial state machine is in the follower state")
	require.Equal(t, follower, controller.stateMachine.State())

	stateObject := controller.object()
	require.NotNil(t, stateObject)
	require.NoError(t, stateObject.Validate())
	require.Equal(t, "ingress-singleton", stateObject.Metadata.Identity.UID)
	require.Equal(t, "ingress-controller", stateObject.Metadata.Name)

	// initial state
	found := Properties{}
	err = stateObject.State.Decode(&found)
	require.NoError(t, err)
	require.Equal(t, Properties{}, found) // initially the state is empty

	// send a tick to the poller
	ticker <- time.Now()
	leader <- true

	<-doneWork

	t.Log("verify state machine moved to the waiting state")
	require.Equal(t, waiting, controller.stateMachine.State())

	// leadership changed
	leader <- false

	ticker <- time.Now()

	t.Log("verify state machine moved to the follower state")

	time.Sleep(500 * time.Millisecond)
	require.Equal(t, follower, controller.stateMachine.State())

	// leadership changed
	leader <- true

	// here we change the routes function to test for another close
	doneWork2 := make(chan interface{})
	controller.routes = func() (map[Vhost][]loadbalancer.Route, error) {
		// if this function is called then we know we've done work in the state transition
		// from syncing to waiting
		close(doneWork2)
		return nil, nil
	}

	ticker <- time.Now()

	t.Log("verify state machine moved to the waiting state")

	<-doneWork2 // if not called, the test will hang here
	require.Equal(t, waiting, controller.stateMachine.State())

	controller.stop()
}
