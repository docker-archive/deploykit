package ingress

import (
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

// This is an unexported interface for testing. The interface
// contains the methods we want to override / mock.  If there's
// a new method on the Controller to be tested and overridden,
// simply add it to this interface and provide an implementation
// below.  This uses composition / embedding in Go to avoid having
// to declare and complicated public interface and use code generations
// like gomock, which has been seen to generate unsafe concurrent code
// (as detected by go vet).
type testControllerInterface interface {
	groupPlugin(plugin.Name) (group.Plugin, error)
	ticker() <-chan time.Time
	init(types.Spec) error
}

type testControllerHarness struct {
	testControllerInterface
	leader       <-chan bool
	_groupPlugin func(plugin.Name) (group.Plugin, error)
	_ticker      func() <-chan time.Time
}

func (tc *testControllerHarness) IsLeader() (bool, error) {
	return <-tc.leader, nil
}

func (tc *testControllerHarness) groupPlugin(name plugin.Name) (group.Plugin, error) {
	return tc._groupPlugin(name)
}

func (tc *testControllerHarness) ticker() <-chan time.Time {
	return tc._ticker()
}

func TestControllerInitSpec(t *testing.T) {

	ticker := make(chan time.Time, 5)
	leader := make(chan bool, 5)
	stopPoller := make(chan interface{})
	close(stopPoller)

	harness := &testControllerHarness{
		leader: leader,
		_groupPlugin: func(n plugin.Name) (group.Plugin, error) {
			t.Log("groupPlugin:", n)
			return nil, nil
		},
		_ticker: func() <-chan time.Time {
			return ticker
		},
	}

	expectedInterval := 10 * time.Second

	realController := &Controller{
		leader:         harness,
		pollerStopChan: stopPoller,
		options: Options{
			SyncInterval: expectedInterval,
		},
	}
	harness.testControllerInterface = realController

	err := harness.init(types.Spec{})
	require.NoError(t, err)

	t.Log("verify that the default value remains despite no Options in the spec")
	require.Equal(t, expectedInterval, realController.options.SyncInterval)

	t.Log("verify that spec's option value makes into the Options")
	realController = &Controller{
		leader: harness,
	}
	harness.testControllerInterface = realController

	expectedOptions := Options{
		HardSync:     true,
		SyncInterval: expectedInterval,
	}

	err = harness.init(types.Spec{
		Options: types.AnyValueMust(expectedOptions),
	})
	require.NoError(t, err)
	require.Equal(t, expectedOptions, realController.options)

	t.Log("verify initial state machine is in the follower state")
	require.Equal(t, follower, realController.stateMachine.State())
}
