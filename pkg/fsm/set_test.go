package fsm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func first(a, b interface{}) interface{} {
	return a
}

func TestSetDeadlineTransition(t *testing.T) {

	const (
		running Index = iota
		wait
	)

	const (
		start Signal = iota
	)

	started := 0
	startAction := func(FSM) error {
		started++
		return nil
	}

	spec, err := Define(
		State{
			Index: wait,
			Transitions: map[Signal]Index{
				start: running,
			},
			Actions: map[Signal]Action{
				start: startAction,
			},
			TTL: Expiry{5, start},
		},
		State{
			Index: running,
		},
	)

	require.NoError(t, err)

	spec.SetStateNames(map[Index]string{
		running: "running",
		wait:    "wait",
	}).SetSignalNames(map[Signal]string{
		start: "start",
	})

	clock := NewClock()

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(spec, clock)

	defer set.Stop()

	// add a few instances
	ids := []ID{}
	states := []Index{}
	instances := []FSM{}

	for i := 0; i < 100; i++ {
		instances = append(instances, set.Add(wait))
	}

	require.Equal(t, 100, set.Size())

	// Expect all 100 to be added to the deadlines queue
	require.Equal(t, 100, set.deadlines.Len())

	// view the instances
	set.ForEach(
		func(id ID, state Index, data interface{}) bool {
			states = append(states, state)
			return false
		},
	)

	// Returning false stops scanning and gets 1
	require.Equal(t, 1, len(states))
	require.Equal(t, wait, states[0])

	// scan again for all entries
	// view the instances
	waiting := 0
	set.ForEach(
		func(id ID, state Index, data interface{}) bool {
			if state == wait {
				waiting++
				return true
			}
			return false
		},
	)
	require.Equal(t, 100, waiting)

	// get the instances
	instances = nil
	for _, id := range ids {
		instances = append(instances, set.Get(id))
	}

	for i, id := range ids {
		require.Equal(t, id, instances[i].ID())
		state := instances[i].State()
		require.Equal(t, wait, state)
	}

	// advance the clock
	clock.Tick() // t = 1

	require.Equal(t, 100, set.CountByState(wait))
	require.Equal(t, 100, set.deadlines.Len())
	require.Equal(t, 100, len(set.bystate[wait]))
	require.Equal(t, 0, len(set.bystate[running]))

	clock.Tick() // t = 2

	// transition a few instances
	for i := 10; i < 20; i++ {

		instance := set.Get(ID(i))

		if state := instance.State(); state == wait {
			require.NoError(t, instance.Signal(start))
		}
	}

	require.Equal(t, 10, set.CountByState(running))
	require.Equal(t, 90, set.CountByState(wait))

	clock.Tick() // t = 3

	require.Equal(t, 10, set.CountByState(running))
	require.Equal(t, 90, set.CountByState(wait))
	require.Equal(t, 10, len(set.bystate[running]))
	require.Equal(t, 90, len(set.bystate[wait]))

	clock.Tick() // t = 4

	require.Equal(t, 10, set.CountByState(running))
	require.Equal(t, 90, set.CountByState(wait))
	require.Equal(t, 10, len(set.bystate[running]))
	require.Equal(t, 90, len(set.bystate[wait]))

	clock.Tick() // t = 5

	time.Sleep(3 * time.Second) // give a little time for the set to settle

	require.Equal(t, 100, set.CountByState(running))
	require.Equal(t, 0, set.CountByState(wait))
	require.Equal(t, 100, len(set.bystate[running]))
	require.Equal(t, 0, len(set.bystate[wait]))
	require.Equal(t, 0, set.deadlines.Len())

	clock.Tick() // t = 6

	require.Equal(t, 100, set.CountByState(running))
	require.Equal(t, 0, set.deadlines.Len())

}

func TestSetFlapping(t *testing.T) {

	const (
		boot Index = iota
		running
		down
		cordoned
	)

	const (
		start Signal = iota
		ping
		timeout
		cordon
	)

	spec, err := Define(
		State{
			Index: boot,
			Transitions: map[Signal]Index{
				start: running,
			},
			TTL: Expiry{3, start},
		},
		State{
			Index: running,
			Transitions: map[Signal]Index{
				timeout: down,
				cordon:  cordoned,
			},
		},
		State{
			Index: down,
			Transitions: map[Signal]Index{
				ping:   running,
				cordon: cordoned,
			},
		},
		State{
			Index: cordoned,
		},
	)
	require.NoError(t, err)

	_, err = spec.CheckFlapping([]Flap{
		{States: [2]Index{running, down}, Count: 3, Raise: cordon},
	})
	require.NoError(t, err)

	clock := NewClock()

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(spec, clock, Options{
		IgnoreUndefinedStates:      true,
		IgnoreUndefinedSignals:     true,
		IgnoreUndefinedTransitions: true,
	})
	defer set.Stop()

	// Add an instance
	instance := set.Add(boot)
	id := instance.ID()

	require.Equal(t, 1, set.Size())
	require.Equal(t, 1, set.CountByState(boot))

	clock.Tick()
	clock.Tick()
	clock.Tick()

	// A slight delay here to let states settle
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 1, set.CountByState(running))
	require.Equal(t, running, instance.State())

	t.Log("************************* running -> down")

	set.Signal(timeout, id) // flap 1 - a

	require.Equal(t, 1, set.CountByState(down))
	require.Equal(t, down, instance.State())
	clock.Tick()

	require.Equal(t, 1, set.CountByState(down))
	require.Equal(t, down, instance.State())

	clock.Tick()

	t.Log("************************* down -> running")

	set.Signal(ping, id) // flap 1 - b

	require.Equal(t, 1, set.CountByState(running))
	require.Equal(t, running, instance.State())

	t.Log("************************* running -> down")

	set.Signal(timeout, id) // flap 2

	require.Equal(t, 1, set.CountByState(down))
	require.Equal(t, down, instance.State())

	t.Log("************************* running -> down")

	require.False(t, instance.CanReceive(timeout))

	err = instance.Signal(timeout)
	require.NoError(t, err) // This does no checking

	t.Log("************************* down -> running")

	set.Signal(ping, id) // flap 2

	t.Log("************************* running -> down")

	set.Signal(timeout, id) // flap 2

	t.Log("************************* down -> running")

	set.Signal(ping, id) // flap 3

	t.Log("************************* running -> down")

	set.Signal(timeout, id) // flap 3

	set.Signal(ping, id) // flap 3

	// note that there's a transition that will be triggered
	time.Sleep(500 * time.Millisecond)

	require.Equal(t, 0, set.CountByState(running))
	require.Equal(t, 1, set.CountByState(cordoned))
	require.Equal(t, cordoned, instance.State())

	set.Stop()
}

func TestMaxVisits(t *testing.T) {
	const (
		up Index = iota
		down
		unavailable
	)

	const (
		startup Signal = iota
		shutdown
		error
	)

	spec, err := Define(
		State{
			Index: up,
			Transitions: map[Signal]Index{
				shutdown: down,
			},
		},
		State{
			Index: down,
			Transitions: map[Signal]Index{
				startup: up,
				error:   unavailable,
			},
			Visit: Limit{2, error},
		},
		State{
			Index: unavailable,
		},
	)

	require.NoError(t, err)

	spec.SetSignalNames(map[Signal]string{
		startup:  "start_up",
		shutdown: "shut_down",
	})

	require.Equal(t, "start_up", spec.SignalName(startup))
	require.Equal(t, "2", spec.SignalName(error))

	spec.SetStateNames(map[Index]string{
		up:   "UP",
		down: "DOWN",
	})

	require.Equal(t, "UP", spec.StateName(up))
	require.Equal(t, "2", spec.StateName(unavailable))

	clock := Wall(time.Tick(1 * time.Second))

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(spec, clock)

	defer set.Stop()

	instance := set.Add(up)

	err = instance.Signal(shutdown)
	require.NoError(t, err)
	require.Equal(t, down, instance.State()) // 1

	err = instance.Signal(startup)
	require.NoError(t, err)
	require.Equal(t, up, instance.State())

	err = instance.Signal(shutdown)
	require.NoError(t, err)
	require.Equal(t, down, instance.State()) // 2

	// then automatically triggered to the unavailable state
	require.Equal(t, unavailable, instance.State())
}

func TestActionErrors(t *testing.T) {
	const (
		up Index = iota
		retrying
		down
		unavailable
	)

	const (
		startup Signal = iota
		shutdown
		warn
		cordon
	)

	spec, err := Define(
		State{
			Index: up,
			Transitions: map[Signal]Index{
				shutdown: down,
			},
		},
		State{
			Index: down,
			Transitions: map[Signal]Index{
				startup: up,
				warn:    retrying,
				cordon:  unavailable,
			},
			Actions: map[Signal]Action{
				startup: func(FSM) error {
					return fmt.Errorf("error")
				},
			},
			Errors: map[Signal]Index{
				startup: retrying,
			},
			Visit: Limit{2, cordon},
		},
		State{
			Index: retrying,
			Transitions: map[Signal]Index{
				warn:    retrying,
				startup: up,
				cordon:  unavailable,
			},
			Actions: map[Signal]Action{
				startup: func(FSM) error {
					return fmt.Errorf("error- retrying")
				},
			},
			Errors: map[Signal]Index{
				startup: retrying,
			},
			Visit: Limit{2, cordon},
		},
		State{
			Index: unavailable,
		},
	)
	require.NoError(t, err)

	spec.SetStateNames(map[Index]string{
		up:          "up",
		retrying:    "retrying",
		down:        "down",
		unavailable: "unavailable",
	}).SetSignalNames(map[Signal]string{
		startup:  "start_up",
		shutdown: "shut_down",
		warn:     "warn",
		cordon:   "cordon",
	})

	clock := Wall(time.Tick(1 * time.Second))

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(spec, clock, Options{
		IgnoreUndefinedTransitions: true,
	})

	defer set.Stop()

	instance := set.Add(up)

	err = instance.Signal(shutdown)
	require.NoError(t, err)
	require.Equal(t, down, instance.State())

	err = instance.Signal(startup)
	require.NoError(t, err)
	require.Equal(t, retrying, instance.State()) // visit 1

	// try 1
	err = instance.Signal(startup)
	require.NoError(t, err)
	require.Equal(t, retrying, instance.State()) // visit 2

	// try 2
	err = instance.Signal(startup)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// then automatically triggered to the unavailable state
	require.Equal(t, unavailable, instance.State())

	t.Log("stopping")
}
