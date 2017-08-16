package leader

import (
	"net/url"
)

// Status indicates leadership status
type Status int

const (
	// NotLeader means the current node is not a leader
	NotLeader Status = iota

	// Leader means the current node / instance is a leader
	Leader

	// Unknown indicates some exception happened while determining leadership.  Consumer will interpret accordingly.
	Unknown
)

// CheckLeaderFunc is all that a special backend needs to implement.  It can be used with the
// NewPoller function to return a polling implementation of the Detector interface.
// This function returns true or false for leadership when there are no errors.  Returned error is reported and
// the status of the event will be set to `Unknown`.
type CheckLeaderFunc func() (bool, error)

// Store is an interface implemented by the environment that supports storing and retrieving the current leader location
type Store interface {

	// UpdateLocation sets the leader to the given URL
	UpdateLocation(*url.URL) error

	// GetLocation returns the location of the current leader.  It's possible that the leader location isn't
	// known despite no errors in querying.  So the url can be nil
	GetLocation() (*url.URL, error)
}

// Leadership is a struct that captures the leadership state, possibly error if exception occurs
type Leadership struct {
	Status Status
	Error  error
}

// Detector is the interface for determining whether this instance is a leader
type Detector interface {

	// Start starts leadership detection
	Start() (<-chan Leadership, error)

	// Receive allocates a channel for the caller to receive on. Messages are sent to all receivers.
	Receive() <-chan Leadership

	// Stop stops
	Stop()
}

// Always is a trivial implementation that asserts the current instance to always be the leader (or not)
func Always(leader bool) CheckLeaderFunc {
	return func() (bool, error) {
		return leader, nil
	}
}
