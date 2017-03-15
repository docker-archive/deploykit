package fsm

// Index is the index of the state in a FSM
type Index int

// Action is the action to take when a signal is received, prior to transition
// to the next state.  The error returned by the function is an exception which
// will put the state machine in an error state.  This error state is not the same
// as some application-specific error state which is a state defined to correspond
// to some external event indicating a real-world error event (as opposed to a
// programming error here).
type Action func(Instance)

// Tick is a unit of time. Time is in relative terms and synchronized with an actual
// timer that's provided by the client.
type Tick int64

// Time is a unit of time not corresponding to wall time
type Time int64

// Expiry specifies the rule for TTL..  A state can have TTL / deadline that when it
// expires a signal can be raised.
type Expiry struct {
	TTL   Tick
	Raise Signal
}

// Signal is a signal that can drive the state machine to transfer from one state to next.
type Signal int

// State encapsulates all the possible transitions and actions to perform during the
// state transition.  A state can have a TTL so that it is allowed to be in that
// state for a given TTL.  On expiration, a signal is raised.
type State struct {
	Index       Index
	Transitions map[Signal]Index
	Actions     map[Signal]Action
	TTL         Expiry
}

// Define performs basic validation, consistency checks and returns a compiled spec.
func Define(s State, more ...State) (spec *Spec, err error) {

	spec = newSpec()

	states := map[Index]State{
		s.Index: s,
	}

	for _, s := range more {
		if _, has := states[s.Index]; has {
			err = errDuplicateState(s.Index)
			return
		}
		states[s.Index] = s
	}

	// check referential integrity
	signals, err := compile(states)
	if err != nil {
		return
	}

	spec.states = states
	spec.signals = signals

	return
}

func compile(m map[Index]State) (map[Signal]Signal, error) {
	signals := map[Signal]Signal{}

	for _, s := range m {

		// Check all the state references in Transitions
		for signal, next := range s.Transitions {
			if _, has := m[next]; !has {
				return nil, unknownState(next)
			}
			signals[signal] = signal
		}

		// Check all the signal references in Actions must be in transitions
		for signal, action := range s.Actions {
			if _, has := s.Transitions[signal]; !has {
				return nil, unknownTransition(signal)
			}

			if action == nil {
				return nil, nilAction(signal)
			}

			if _, has := signals[signal]; !has {
				return nil, unknownSignal(signal)
			}
		}

	}
	return signals, nil
}

// Limit is a numerical value indicating a limit of occurrences.
type Limit int

// Spec is a specification of all the rules for the fsm
type Spec struct {
	states  map[Index]State
	signals map[Signal]Signal
	flaps   map[[2]Index]*Flap
}

func newSpec() *Spec {
	return &Spec{
		states:  map[Index]State{},
		signals: map[Signal]Signal{},
		flaps:   map[[2]Index]*Flap{},
	}
}

// returns an expiry for the state.  if the TTL is 0 then there's no expiry for the state.
func (s *Spec) expiry(state Index) (expiry Expiry, found bool) {
	st, has := s.states[state]
	if !has {
		found = false
		return
	}
	if st.TTL.TTL > 0 {
		expiry = st.TTL
		found = true
	}
	return
}

// transition takes the fsm from a current state, with given signal, to the next state.
// returns error if the transition is not possible.
func (s *Spec) transition(current Index, signal Signal) (next Index, action Action, err error) {
	state, has := s.states[current]
	if !has {
		err = unknownState(current)
		return
	}

	if len(state.Transitions) == 0 {
		err = noTransitions(*s)
		return
	}

	_, has = s.signals[signal]
	if !has {
		err = unknownSignal(signal)
		return
	}

	n, has := state.Transitions[signal]
	if !has {
		err = unknownTransition(signal)
		return
	}
	next = n

	if a, has := state.Actions[signal]; has {
		action = a
	}

	return
}

// Flap is oscillation between two adjacent states.  For example, a->b followed by b->a is
// counted as 1 flap.  Similarly, b->a followed by a->b is another flap.
type Flap struct {
	States [2]Index
	Count  int
	Raise  Signal
}

func (s *Spec) flap(a, b Index) *Flap {
	key := [2]Index{a, b}
	if a > b {
		key = [2]Index{b, a}
	}
	if f, has := s.flaps[key]; has {
		return f
	}
	return nil
}

// CheckFlappingMust is a Must version (will panic if err) of CheckFlapping
func (s *Spec) CheckFlappingMust(checks []Flap) *Spec {
	_, err := s.CheckFlapping(checks)
	if err != nil {
		panic(err)
	}
	return s
}

// CheckFlapping - Limit is the maximum of a->b b->a transitions allowable.  For detecting
// oscillations between two adjacent states (no hops)
func (s *Spec) CheckFlapping(checks []Flap) (*Spec, error) {
	flaps := map[[2]Index]*Flap{}
	for _, check := range checks {

		// check the state
		for _, state := range check.States {
			if _, has := s.states[state]; !has {
				return nil, unknownState(state)
			}
		}

		key := [2]Index{check.States[0], check.States[1]}
		if check.States[0] > check.States[1] {
			key = [2]Index{check.States[1], check.States[0]}
		}

		copy := check
		flaps[key] = &copy
	}

	s.flaps = flaps

	return s, nil
}
