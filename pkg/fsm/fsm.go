package fsm

import (
	"fmt"
)

// With returns a spec with given state names and signal names
func With(stateNames map[Index]string, signalNames map[Signal]string) *Spec {
	spec := newSpec()
	return spec.SetStateNames(stateNames).SetSignalNames(signalNames)
}

// Define performs basic validation, consistency checks and returns a compiled spec.
func (s *Spec) Define(state State, more ...State) (*Spec, error) {
	states := map[Index]State{
		state.Index: state,
	}

	for _, st := range more {
		if _, has := states[st.Index]; has {
			err := ErrDuplicateState(st.Index)
			return s, err
		}
		states[st.Index] = st
	}

	// check referential integrity
	signals, err := s.compile(states)
	if err != nil {
		return s, err
	}

	s.states = states
	s.signals = signals
	return s, err
}

// Define performs basic validation, consistency checks and returns a compiled spec.
func Define(s State, more ...State) (spec *Spec, err error) {
	spec = newSpec()
	return spec.Define(s, more...)
}

func (s *Spec) compile(m map[Index]State) (map[Signal]Signal, error) {

	signals := map[Signal]Signal{}

	for _, st := range m {
		for _, transfer := range []map[Signal]Index{
			st.Transitions,
			st.Errors,
		} {
			for signal, next := range transfer {
				if _, has := m[next]; !has {
					log.Error("unknown state", "next", s.StateName(next))
					return nil, ErrUnknownState(next)
				}
				signals[signal] = signal
			}
		}
	}

	// all signals must be known here

	for _, st := range m {
		// Check all the signal references in Actions must be in transitions
		for signal, action := range st.Actions {
			if _, has := st.Transitions[signal]; !has {
				log.Error("actions has signal that's not in state's transitions",
					"state", s.StateName(st.Index), "signal", s.SignalName(signal))
				return nil, ErrUnknownTransition{spec: s, Signal: signal, State: st.Index}
			}

			if action == nil {
				return nil, ErrNilAction(signal)
			}

			if _, has := signals[signal]; !has {
				return nil, ErrUnknownSignal{Signal: signal, State: st.Index}
			}
		}
	}

	// what's raised in the TTL and in the Visit limit must be defined as well

	for _, st := range m {
		if st.TTL.TTL > 0 {
			if _, has := st.Transitions[st.TTL.Raise]; !has {
				log.Error("expiry raises signal that's not in state's transitions",
					"state", s.StateName(st.Index), "TTL", st.TTL)
				return nil, ErrUnknownSignal{spec: s, Signal: st.TTL.Raise, State: st.Index}
			}

			// register as valid signal
			signals[st.TTL.Raise] = st.TTL.Raise

		}
		if st.Visit.Value > 0 {
			if _, has := st.Transitions[st.Visit.Raise]; !has {
				log.Error("visit limit raises signal that's not in state's transitions",
					"state", s.StateName(st.Index), "visit", st.Visit)
				return nil, ErrUnknownSignal{spec: s, Signal: st.Visit.Raise, State: st.Index}
			}

			// register as valid signal
			signals[st.Visit.Raise] = st.Visit.Raise
		}
	}

	return signals, nil
}

// Spec is a specification of all the rules for the fsm
type Spec struct {
	states  map[Index]State
	signals map[Signal]Signal
	flaps   map[[2]Index]*Flap

	stateNames  map[Index]string  // optional
	signalNames map[Signal]string // optional
}

func newSpec() *Spec {
	return &Spec{
		states:  map[Index]State{},
		signals: map[Signal]Signal{},
		flaps:   map[[2]Index]*Flap{},
	}
}

// SetStateNames sets the friendly names for the states
func (s *Spec) SetStateNames(v map[Index]string) *Spec {
	s.stateNames = v
	return s
}

// SetSignalNames sets the friendly names for the signals
func (s *Spec) SetSignalNames(v map[Signal]string) *Spec {
	s.signalNames = v
	return s
}

// StateName returns the friendly name of the state, if defined
func (s *Spec) StateName(i Index) (name string) {
	name = fmt.Sprintf("%v", i)
	if s == nil {
		return
	}
	if s.stateNames == nil {
		return
	}
	if v, has := s.stateNames[i]; has {
		name = v
	}
	return
}

// SignalName returns the friendly name of the signal, if defined
func (s *Spec) SignalName(signal Signal) (name string) {
	name = fmt.Sprintf("%v", signal)
	if s == nil {
		return
	}

	if s.signalNames == nil {
		return
	}
	if v, has := s.signalNames[signal]; has {
		name = v
	}
	return
}

// SetAction sets the action associated with a signal in a given state
func (s *Spec) SetAction(state Index, signal Signal, action Action) error {
	st, has := s.states[state]
	if !has {
		return fmt.Errorf("no such state %v", state)
	}
	if st.Actions == nil {
		st.Actions = map[Signal]Action{}
	}
	st.Actions[signal] = action
	s.states[state] = st // Update the map because the map returned a copy of the state.
	return nil
}

// returns an expiry for the state.  if the TTL is 0 then there's no expiry for the state.
func (s *Spec) expiry(current Index) (expiry *Expiry, err error) {
	state, has := s.states[current]
	if !has {
		err = ErrUnknownState(current)
		return
	}
	if state.TTL.TTL > 0 {
		expiry = &state.TTL
	}
	return
}

// returns the limit on visiting this state
func (s *Spec) visit(next Index) (limit *Limit, err error) {
	state, has := s.states[next]
	if !has {
		err = ErrUnknownState(next)
		return
	}

	if state.Visit.Value > 0 {
		limit = &state.Visit
	}
	return
}

// returns an error handling rule
func (s *Spec) error(current Index, signal Signal) (next Index, err error) {
	state, has := s.states[current]
	if !has {
		err = ErrUnknownState(current)
		return
	}

	_, has = s.signals[signal]
	if !has {
		err = ErrUnknownSignal{Signal: signal, State: current}
		return
	}

	v, has := state.Errors[signal]
	if !has {
		err = ErrUnknownTransition{Signal: signal, State: current}
		return
	}
	next = v
	return
}

// transition takes the fsm from a current state, with given signal, to the next state.
// returns error if the transition is not possible.
func (s *Spec) transition(current Index, signal Signal) (next Index, action Action, err error) {

	next = -1
	defer func() {
		log.Debug("transition:",
			"current", s.StateName(current),
			"signal", s.SignalName(signal),
			"next", s.StateName(next),
			"action", action, "err", err, "V", debugV2)
	}()

	state, has := s.states[current]
	if !has {
		err = ErrUnknownState(current)
		return
	}

	if len(state.Transitions) == 0 {
		err = ErrNoTransitions(*s)
		return
	}

	_, has = s.signals[signal]
	if !has {
		err = ErrUnknownSignal{Signal: signal}
		return
	}

	n, has := state.Transitions[signal]
	if !has {
		err = ErrUnknownTransition{Signal: signal, State: state.Index}
		return
	}
	next = n

	if a, has := state.Actions[signal]; has {
		action = a
	}

	return
}
