package libmachete

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
