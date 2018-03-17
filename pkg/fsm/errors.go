package fsm

import (
	"fmt"
)

// ErrDuplicateState is thrown when there are indexes of the same value
type ErrDuplicateState Index

func (e ErrDuplicateState) Error() string {
	return fmt.Sprintf("duplicated state index: %v", e)
}

// ErrUnknownState indicates the state referenced does not match a known state index
type ErrUnknownState Index

func (e ErrUnknownState) Error() string {
	return fmt.Sprintf("unknown state: %v", e)
}

// ErrUnknownTransition indicates an unknown signal while in given state is raised
type ErrUnknownTransition struct {
	spec   *Spec
	Signal Signal
	State  Index
}

func (e ErrUnknownTransition) Error() string {
	return fmt.Sprintf("unknown stransition: signal=%v, state=%v", e.spec.SignalName(e.Signal), e.spec.StateName(e.State))
}

// ErrUnknownSignal is raised when a undefined signal is received in the given state
type ErrUnknownSignal struct {
	spec   *Spec
	Signal Signal
	State  Index
}

func (e ErrUnknownSignal) Error() string {
	return fmt.Sprintf("unknown signal: signal=%v, state=%v", e.spec.SignalName(e.Signal), e.spec.StateName(e.State))
}

// ErrUnknownFSM is raised when the ID is does not match any thing in the set
type ErrUnknownFSM ID

func (e ErrUnknownFSM) Error() string {
	return fmt.Sprintf("unknown instance: %v", e)
}

// ErrNilAction is raised when an action is nil
type ErrNilAction Signal

func (e ErrNilAction) Error() string {
	return fmt.Sprintf("nil action corresponding to signal %d", e)
}

// ErrNoTransitions is raised when there are no transitions defined
type ErrNoTransitions Spec

func (e ErrNoTransitions) Error() string {
	return fmt.Sprintf("no transitions defined: count(states)=%d", len(e.states))
}
