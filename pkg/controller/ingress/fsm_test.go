package ingress

import (
	"fmt"
	"testing"
	"time"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestProcessSpecFSMUsage(t *testing.T) {

	clock := fsm.NewClock()

	runSync := make(chan interface{})
	stateMachineSpec.SetAction(syncing, sync, func(i fsm.FSM) error {
		close(runSync)
		return nil
	})

	instances := fsm.NewSet(stateMachineSpec, clock)

	// initial state of the controller is to be in follwer state.
	obj := instances.Add(follower)
	require.NotNil(t, obj)

	require.NoError(t, obj.Signal(follow))
	require.NoError(t, obj.Signal(start))

	// we are still in the follower state
	require.Equal(t, follower, obj.State())

	// the controller decides that it's the leader
	err := obj.Signal(lead)
	require.NoError(t, err)

	require.Equal(t, waiting, obj.State())

	// after some time, a timer tells the controller to sync
	require.NoError(t, obj.Signal(start))
	require.NoError(t, obj.Signal(sync))

	<-runSync // moves on only if we call the action

	// after sync we are in the waiting state
	require.Equal(t, waiting, obj.State())

	require.NoError(t, obj.Signal(follow))
	require.Equal(t, follower, obj.State())

	// Note that we should check to see if the signal is valid
	require.False(t, obj.CanReceive(sync))
	require.False(t, obj.CanReceive(start))

	// leader again
	require.NoError(t, obj.Signal(lead))
	require.Equal(t, waiting, obj.State())

	// timer wakes up
	require.NoError(t, obj.Signal(start))
	require.Equal(t, syncing, obj.State())
	require.False(t, obj.CanReceive(start))
	require.True(t, obj.CanReceive(sync))
	require.True(t, obj.CanReceive(follow))

	// oops not a leader now
	require.NoError(t, obj.Signal(follow))
	require.Equal(t, follower, obj.State())

	require.False(t, obj.CanReceive(sync))
	require.False(t, obj.CanReceive(start))
	require.True(t, obj.CanReceive(lead))
}

func TestMustTrue(t *testing.T) {
	require.True(t, mustTrue(func() (bool, error) { return true, nil }()))
	require.False(t, mustTrue(func() (bool, error) { return true, fmt.Errorf("error") }()))
	require.False(t, mustTrue(func() (bool, error) { return false, nil }()))
}

func TestControllerInitSpec(t *testing.T) {
	expectedInterval := 10 * time.Second

	managedObject := &managed{
		options: ingress.Options{
			SyncInterval: types.Duration(expectedInterval),
		},
	}

	err := managedObject.init(types.Spec{})
	require.NoError(t, err)

	t.Log("verify that the default value remains despite no Options in the spec")
	require.Equal(t, expectedInterval, managedObject.options.SyncInterval.Duration())

	t.Log("verify that spec's option value makes into the ingress.Options")
	managedObject = &managed{}

	expectedOptions := ingress.Options{
		HardSync:     true,
		SyncInterval: types.Duration(expectedInterval),
	}

	err = managedObject.init(types.Spec{
		Options: types.AnyValueMust(expectedOptions),
	})
	require.NoError(t, err)
	require.Equal(t, expectedOptions, managedObject.options)

	t.Log("verify initial state machine is in the follower state")
	require.Equal(t, follower, managedObject.stateMachine.State())
}
