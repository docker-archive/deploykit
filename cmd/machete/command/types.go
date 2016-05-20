package command

import "os"

// HandledError is a command error that will report and terminate itself.
type HandledError interface {
	Handle()
}

type osExitError struct {
	text     string
	exitCode int
}

func (e *osExitError) Handle() {
	os.Exit(e.exitCode)
}

func (e *osExitError) Error() string {
	return e.text
}

// UsageError indicates that the command was used improperly.
var UsageError = &osExitError{exitCode: -1, text: "incorrect usage"}

// NotImplementedError indicates that the requested behavior is not yet supported.
var NotImplementedError = &osExitError{exitCode: -2, text: "Not implemented"}
