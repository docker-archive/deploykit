package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetDeadlineTransition(t *testing.T) {

	const (
		running Index = iota
		wait
	)

	const (
		start Signal = iota
	)

	started := 0
	startAction := func(Instance) {
		started++
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

	clock := NewClock()

	// set is a collection of fsm intances that follow the same rules.
	set := NewSet(spec, clock)

	defer set.Stop()

	// add a few instances
	ids := []ID{}
	states := []Index{}
	instances := []Instance{}

	for i := 0; i < 100; i++ {
		instances = append(instances, set.Add(wait))
	}

	require.Equal(t, 100, set.Size())

	// Expect all 100 to be added to the deadlines queue
	require.Equal(t, 100, set.deadlines.Len())

	// view the instances
	set.ForEachInstance(
		func(id ID, state Index) bool {
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
	set.ForEachInstance(
		func(id ID, state Index) bool {
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
		instances = append(instances, set.Instance(id))
	}

	for i, id := range ids {
		require.Equal(t, id, instances[i].ID())
		state, ok := instances[i].State()
		require.True(t, ok)
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

		instance := set.Instance(ID(i))

		if state, ok := instance.State(); ok && state == wait {
			require.NoError(t, instance.Signal(start))
		}
	}

	time.Sleep(1 * time.Second)

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

	time.Sleep(1 * time.Second) // give a little time for the set to settle
	require.Equal(t, 100, set.CountByState(running))
	require.Equal(t, 0, set.CountByState(wait))
	require.Equal(t, 100, len(set.bystate[running]))
	require.Equal(t, 0, len(set.bystate[wait]))
	require.Equal(t, 0, set.deadlines.Len())

	clock.Tick() // t = 6

	require.Equal(t, 100, set.CountByState(running))
	require.Equal(t, 0, set.deadlines.Len())

}
