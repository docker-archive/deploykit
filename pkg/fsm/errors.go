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
