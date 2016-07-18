package spi

import "fmt"

const (
	// ErrUnknown is the default error code.  This should generally not be used, or should be reserved for cases
	// where it is not possible to offer additional information on the error.
	ErrUnknown int = iota

	// ErrDuplicate is returned when an attempt is made to create an object that already exists.
	ErrDuplicate

	// ErrNotFound indicates that a referenced object does not exist by key
	ErrNotFound

	// ErrBadInput is returned when an operation was supplied invalid user input.
	ErrBadInput
)

// Error is the libmachete specific error
type Error struct {
	// Code is the error code
	Code int

	// Message is the error message
	Message string
}

// Implements the Error interface.
func (e Error) Error() string {
	return e.Message
}

// UnknownError creates a standard Error when the cause is unknown.
func UnknownError(err error) error {
	return Error{ErrUnknown, err.Error()}
}

// NewError creates an Error with the specified code.
func NewError(code int, format string, args ...interface{}) error {
	return Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeFromError returns the code associated with an error.
// Returns ErrUnknown if the error is not an spi error.
func CodeFromError(err error) int {
	spiErr, is := err.(Error)
	if !is {
		return ErrUnknown
	}
	return spiErr.Code
}
