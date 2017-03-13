package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefinition(t *testing.T) {

	const (
		turnOn Signal = iota
		turnOff

		on Index = iota
		off
	)

	m := map[Index]State{
		on: {
			Index: on,
			Transitions: map[Signal]Index{
				turnOff: off,
			},
		},
	}

	_, err := compile(m)
	require.Error(t, err)

	// add missing
	m[off] = State{
		Index: off,
		Transitions: map[Signal]Index{
			turnOn: on,
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
		turnOn Signal = iota
		turnOff
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
				turnOn: on,
			},
			Actions: map[Signal]Action{
				turnOn: sayHi,
			},
		},
		State{
			Index: on,
			Transitions: map[Signal]Index{
				turnOff: sleep,
				unplug:  off,
			},
			Actions: map[Signal]Action{
				turnOff: sayBye,
			},
		},
		State{
			Index: sleep,
			Transitions: map[Signal]Index{
				turnOn:  on,
				turnOff: off,
				unplug:  off,
			},
			Actions: map[Signal]Action{
				turnOn:  sayHi,
				turnOff: sayBye,
			},
		},
	)

	require.NoError(t, err)

	// check transitions
	next, action, err := spec.transition(on, turnOff)
	require.NoError(t, err)
	require.Equal(t, sleep, next)
	action(nil)
	<-saidBye

	// check transitions
	next, action, err = spec.transition(off, turnOn)
	require.NoError(t, err)
	require.Equal(t, on, next)
	action(nil)
	<-saidHi

	// not allowed transition
	_, _, err = spec.transition(on, turnOn)
	require.Error(t, err)
}

func TestFsmUsage(t *testing.T) {

	const (
		signalSpecified Signal = iota
		signalCreate
		signalFound
		signalHealthy
		signalUnhealthy
		signalStartOver
		signalStop

		specified Index = iota
		creating
		up
		running
		down
		decommissioned
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
			Index: specified,
			Transitions: map[Signal]Index{
				signalCreate:    creating,
				signalFound:     up,
				signalHealthy:   running,
				signalUnhealthy: down,
			},
			Actions: map[Signal]Action{
				signalCreate: createInstance,
			},
			TTL: Expiry{1000, signalCreate},
		},
		State{
			Index: creating,
			Transitions: map[Signal]Index{
				signalFound:     up,
				signalStartOver: specified,
			},
			Actions: map[Signal]Action{
				signalStartOver: cleanup,
			},
			TTL: Expiry{1000, signalStartOver},
		},
		State{
			Index: up,
			Transitions: map[Signal]Index{
				signalHealthy:   running,
				signalUnhealthy: down,
			},
			Actions: map[Signal]Action{
				signalUnhealthy: recordFlapping, // note flapping between up and down
			},
		},
		State{
			Index: down,
			Transitions: map[Signal]Index{
				signalStartOver: specified,
				signalHealthy:   running,
			},
			Actions: map[Signal]Action{
				signalStartOver: cleanup,
				signalHealthy:   recordFlapping, // note flapping between up and down
			},
			TTL: Expiry{10, signalStartOver},
		},
		State{
			Index: running,
			Transitions: map[Signal]Index{
				signalHealthy:   running,
				signalUnhealthy: down, // do we want threshold e.g. more than N signals?
				signalStop:      decommissioned,
			},
			Actions: map[Signal]Action{
				signalUnhealthy: sendAlert,
				signalStop:      deleteInstance,
			},
		},
		State{
			Index: decommissioned,
		},
	)

	require.NoError(t, err)

	clock := Wall(time.Tick(1 * time.Second))

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(fsm.CheckFlappingMust([]Flap{
		{States: [2]Index{running, down}, Count: 10},
	}), clock)

	// allocates a new instance of a fsm with an initial state.
	instance := set.Add(specified)
	require.NotNil(t, instance)

	set.Stop()
}
