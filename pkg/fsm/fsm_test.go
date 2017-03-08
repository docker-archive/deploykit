package fsm

import (
	"testing"
	"time"

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

	_, err := compile(m)
	require.Error(t, err)

	// add missing
	m[off] = State{
		Index: off,
		Transitions: map[Signal]Index{
			turn_on: on,
		},
	}

	_, err = compile(m)
	require.NoError(t, err)

	states := []State{}
	for _, s := range m {
		states = append(states, s)
	}

	spec, err := Define(states[0], states[1:]...)
	require.NoError(t, err)
	require.Equal(t, 2, len(spec.signals))
	require.Equal(t, 2, len(spec.states))

	spec = spec.CheckFlappingMust([]Flap{
		{States: [2]Index{on, off}, Count: 100},
	})

	require.Equal(t, 1, len(spec.flaps))
	t.Log(spec)
}

func TestSimple(t *testing.T) {

	const (
		on Index = iota
		off
		sleep
	)

	const (
		turn_on Signal = iota
		turn_off
		unplug
	)

	saidHi := make(chan struct{})
	var sayHi Action = func(Instance) {
		close(saidHi)
	}
	saidBye := make(chan struct{})
	var sayBye Action = func(Instance) {
		close(saidBye)
	}

	spec, err := Define(
		State{
			Index: off,
			Transitions: map[Signal]Index{
				turn_on: on,
			},
			Actions: map[Signal]Action{
				turn_on: sayHi,
			},
		},
		State{
			Index: on,
			Transitions: map[Signal]Index{
				turn_off: sleep,
				unplug:   off,
			},
			Actions: map[Signal]Action{
				turn_off: sayBye,
			},
		},
		State{
			Index: sleep,
			Transitions: map[Signal]Index{
				turn_on:  on,
				turn_off: off,
				unplug:   off,
			},
			Actions: map[Signal]Action{
				turn_on:  sayHi,
				turn_off: sayBye,
			},
		},
	)

	require.NoError(t, err)

	// check transitions
	next, action, err := spec.transition(on, turn_off)
	require.NoError(t, err)
	require.Equal(t, sleep, next)
	action(nil)
	<-saidBye

	// check transitions
	next, action, err = spec.transition(off, turn_on)
	require.NoError(t, err)
	require.Equal(t, on, next)
	action(nil)
	<-saidHi

	// not allowed transition
	_, _, err = spec.transition(on, turn_on)
	require.Error(t, err)
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

	createInstance := func(Instance) {
		t.Log("creating instance")
	}
	deleteInstance := func(Instance) {
		t.Log("delete instance")
	}
	cleanup := func(Instance) {
		t.Log("cleanup")
	}
	recordFlapping := func(Instance) {
		t.Log("flap is if this happens more than multiples of 2 calls")
	}
	sendAlert := func(Instance) {
		t.Log("alert")
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
	set := NewSet(fsm.CheckFlappingMust([]Flap{
		{States: [2]Index{state_running, state_down}, Count: 10},
	}), time.Tick(1*time.Second))

	// allocates a new instance of a fsm with an initial state.
	instance := set.Add(state_specified)

	require.NotNil(t, instance)

}
