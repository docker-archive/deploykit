package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefinition(t *testing.T) {

	const (
		turn_on Signal = iota
		turn_off

		on Index = iota
		off
	)

	m := map[Index]State{
		on: {
			Index: on,
			Transitions: map[Signal]Index{
				turn_off: off,
			},
		},
	}

	require.Error(t, checkReferences(m))

	// add missing
	m[off] = State{
		Index: off,
		Transitions: map[Signal]Index{
			turn_on: on,
		},
	}

	require.NoError(t, checkReferences(m))

	states := []State{}
	for _, s := range m {
		states = append(states, s)
	}

	spec, err := Define(states[0], states[1:]...)
	require.NoError(t, err)

	spec = spec.CheckFlapping([]Flap{
		{States: [2]Index{on, off}, Count: 100},
	})

	t.Log(spec)
}

func TestFsmUsage(t *testing.T) {

	const (
		signal_specified Signal = iota
		signal_create
		signal_found
		signal_healthy
		signal_unhealthy
		signal_startover
		signal_stop

		state_specified Index = iota
		state_creating
		state_up
		state_running
		state_down
		state_decommissioned
	)

	createInstance := func() error {
		t.Log("creating instance")
		return nil
	}
	deleteInstance := func() error {
		t.Log("delete instance")
		return nil
	}
	cleanup := func() error {
		t.Log("cleanup")
		return nil
	}
	recordFlapping := func() error {
		t.Log("flap is if this happens more than multiples of 2 calls")
		return nil
	}
	sendAlert := func() error {
		t.Log("alert")
		return nil
	}

	fsm, err := Define(
		State{
			Index: state_specified,
			Transitions: map[Signal]Index{
				signal_create:    state_creating,
				signal_found:     state_up,
				signal_healthy:   state_running,
				signal_unhealthy: state_down,
			},
			Actions: map[Signal]Action{
				signal_create: createInstance,
			},
			TTL: Expiry{1000, signal_create},
		},
		State{
			Index: state_creating,
			Transitions: map[Signal]Index{
				signal_found:     state_up,
				signal_startover: state_specified,
			},
			Actions: map[Signal]Action{
				signal_startover: cleanup,
			},
			TTL: Expiry{1000, signal_startover},
		},
		State{
			Index: state_up,
			Transitions: map[Signal]Index{
				signal_healthy:   state_running,
				signal_unhealthy: state_down,
			},
			Actions: map[Signal]Action{
				signal_unhealthy: recordFlapping, // note flapping between up and down
			},
		},
		State{
			Index: state_down,
			Transitions: map[Signal]Index{
				signal_startover: state_specified,
				signal_healthy:   state_running,
			},
			Actions: map[Signal]Action{
				signal_startover: cleanup,
				signal_healthy:   recordFlapping, // note flapping between up and down
			},
			TTL: Expiry{10, signal_startover},
		},
		State{
			Index: state_running,
			Transitions: map[Signal]Index{
				signal_healthy:   state_running,
				signal_unhealthy: state_down, // do we want threshold e.g. more than N signals?
				signal_stop:      state_decommissioned,
			},
			Actions: map[Signal]Action{
				signal_unhealthy: sendAlert,
				signal_stop:      deleteInstance,
			},
		},
		State{
			Index: state_decommissioned,
		},
	)

	require.NoError(t, err)

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(fsm.CheckFlapping([]Flap{
		{States: [2]Index{state_running, state_down}, Count: 10},
	}))

	// allocates a new instance of a fsm with an initial state.
	instance := set.New(state_specified)

	require.NotNil(t, instance)

}
