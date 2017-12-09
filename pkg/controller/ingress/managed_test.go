package ingress

import (
	"net/url"
	"testing"
	"time"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func fakeLeadership(c <-chan bool) func() stack.Leadership {
	return func() stack.Leadership { return fakeLeadershipT(c) }
}

type fakeLeadershipT <-chan bool

func (l fakeLeadershipT) IsLeader() (bool, error) {
	return <-l, nil
}

func (l fakeLeadershipT) LeaderLocation() (*url.URL, error) {
	return nil, nil
}

func TestManagedStartStop(t *testing.T) {

	ticker := make(chan time.Time, 1)
	leader := make(chan bool, 1)

	doneWork := make(chan int, 1)
	managedObject := &managed{
		scope:        scope.Nil,
		leader:       fakeLeadership(leader),
		ticker:       ticker,
		healthChecks: func() (map[ingress.Vhost][]loadbalancer.HealthCheck, error) { return nil, nil },
		groups:       func() (map[ingress.Vhost][]ingress.Group, error) { return nil, nil },
		l4s:          func() (map[ingress.Vhost]loadbalancer.L4, error) { return nil, nil },

		routes: func() (map[ingress.Vhost][]loadbalancer.Route, error) {
			// if this function is called then we know we've done work in the state transition
			doneWork <- 1
			return nil, nil
		},
	}

	spec := types.Spec{
		Kind:    "ingress-controller",
		Version: "0.1",
		Metadata: types.Metadata{
			Name: "ingress-controller",
		},
		Properties: types.AnyValueMust(ingress.Properties{
			{
				Vhost:    ingress.Vhost("default"),
				L4Plugin: plugin.Name("elb-1"),
			},
		}),
	}
	err := managedObject.init(spec)
	require.NoError(t, err)

	managedObject.Start()

	t.Log("verify initial state machine is in the follower state")
	require.Equal(t, follower, managedObject.stateMachine.State())

	stateObject := managedObject.object()
	require.NotNil(t, stateObject)
	require.NoError(t, stateObject.Validate())
	require.Equal(t, "ingress-singleton", stateObject.Metadata.Identity.ID)
	require.Equal(t, "ingress-controller", stateObject.Metadata.Name)

	// initial state
	found := ingress.Properties{}
	err = stateObject.State.Decode(&found)
	require.NoError(t, err)
	require.Equal(t, ingress.Properties{}, found) // initially the state is empty

	// send a tick to the poller
	ticker <- time.Now()
	leader <- true

	<-doneWork

	t.Log("verify state machine moved to the waiting state")
	require.Equal(t, waiting, managedObject.stateMachine.State())

	// leadership changed
	leader <- false

	ticker <- time.Now()

	t.Log("verify state machine moved to the follower state")

	time.Sleep(500 * time.Millisecond)
	require.Equal(t, follower, managedObject.stateMachine.State())

	// leadership changed
	leader <- true

	// here we change the routes function to test for another close

	ticker <- time.Now()

	<-doneWork // if not called, the test will hang here

	t.Log("verify state machine moved to the waiting state")
	require.Equal(t, waiting, managedObject.stateMachine.State())

	managedObject.Stop()

}
