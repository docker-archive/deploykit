package fsm

import (
	"sort"
)

type Index int

// Action is the action to take when a signal is received, prior to transition
// to the next state.  The error returned by the function is an exception which
// will put the state machine in an error state.  This error state is not the same
// as some application-specific error state which is a state defined to correspond
// to some external event indicating a real-world error event (as opposed to a
// programming error here).
type Action func() error

// Tick is a unit of time. Time is in relative terms and synchronized with an actual
// timer that's provided by the client.
type Tick int64

type Expiry struct {
	TTL   Tick
	Raise Signal
}

type State struct {
	Index       Index
	Transitions map[Signal]Index
	Actions     map[Signal]Action
	TTL         Expiry
}

type Signal int

func Define(s State, more ...State) (spec Spec, err error) {
	m := map[Index]State{
		s.Index: s,
	}

	for _, s := range more {
		if _, has := m[s.Index]; has {
			err = errDuplicateState(s.Index)
			return
		}
		m[s.Index] = s
	}

	// check referential integrity
	if err = checkReferences(m); err != nil {
		return
	}

	spec.states = m
	return
}

func checkReferences(m map[Index]State) error {
	for _, s := range m {
		for _, i := range s.Transitions {
			if _, has := m[i]; !has {
				return unknownState(i)
			}
		}
	}
	return nil
}

type Limit int

type Spec struct {
	states   map[Index]State
	maxFlaps map[[2]Index]Limit
}

type Flap struct {
	States [2]Index
	Count  int
}

// CheckFlapping - Limit is the maximum of a->b b->a transitions allowable.  For detecting
// oscillations between two adjacent states (no hops)
func (spec *Spec) CheckFlapping(checks []Flap) Spec {
	if spec.maxFlaps == nil {
		spec.maxFlaps = map[[2]Index]Limit{}
	}
	for _, check := range checks {
		copy := []int{int(check.States[0]), int(check.States[1])}
		sort.Ints(copy)
		spec.maxFlaps[[2]Index{Index(copy[0]), Index(copy[1])}] = Limit(check.Count)
	}
	return *spec
}

type set struct {
	spec Spec
}

func NewSet(spec Spec) *set {
	return &set{
		spec: spec,
	}
}

func (s *set) New(initial Index) Instance {
	return &instance{
		state:  initial,
		parent: s,
	}
}

type Instance interface {
	Signal(Signal) error
}

type instance struct {
	state  Index
	parent *set
	error  error
}

func (i instance) Signal(s Signal) error {
	return nil
}
