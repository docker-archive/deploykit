package libmachete

import (
	"fmt"
)

const (
	// ErrDuplicate indicates duplicate object by key
	ErrDuplicate int = iota
	// ErrNotFound indicates object does not exist by key
	ErrNotFound
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

// NewError creates an Error with the specified code.
func NewError(code int, format string, args ...interface{}) error {
	return Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// IsErr returns whether the error is an Error and has the specified code
func IsErr(e error, code int) bool {
	if err, ok := e.(Error); ok {
		return err.Code == code
	}
	return false
}
