package fsm

import (
	"fmt"
)

type errDuplicateState Index

func (e errDuplicateState) Error() string {
	return fmt.Sprintf("duplicated state index: %d", e)
}

type unknownState Index

func (e unknownState) Error() string {
	return fmt.Sprintf("unknown state index: %d", e)
}

type unknownTransition struct {
	signal Signal
	state  Index
}

func (e unknownTransition) Error() string {
	return fmt.Sprintf("state %d - no transition defined for signal %d", e.state, e.signal)
}

type unknownSignal struct {
	signal Signal
	state  Index
}

func (e unknownSignal) Error() string {
	return fmt.Sprintf("state %d - signal in action not found in transitions %d", e.state, e.signal)
}

type unknownInstance ID

func (e unknownInstance) Error() string {
	return fmt.Sprintf("unknown instance %d", e)
}

type nilAction Signal

func (e nilAction) Error() string {
	return fmt.Sprintf("nil action corresponding to signal %d", e)
}

type noTransitions Spec

func (e noTransitions) Error() string {
	return fmt.Sprintf("no transitions defined: count(states)=%d", len(e.states))
}
