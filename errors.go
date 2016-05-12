package libmachete

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
