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

type unknownTransition Signal

func (e unknownTransition) Error() string {
	return fmt.Sprintf("no transition defined for signal %d", e)
}

type unknownSignal Signal

func (e unknownSignal) Error() string {
	return fmt.Sprintf("signal in action not found in transitions %d", e)
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
	return fmt.Sprintf("no transitions defined: states=%d", len(e.states))
}
