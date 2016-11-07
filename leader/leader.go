package leader

// Status indicates leadership status
type Status int

const (
	// StatusNotLeader means the current node is not a leader
	StatusNotLeader Status = iota

	// StatusLeader means the current node / instance is a leader
	StatusLeader

	// StatusUnknown indicates some exception happened while determining leadership.  Consumer will interpret accordingly.
	StatusUnknown
)

// CheckLeaderFunc is all that a special backend needs to implement.  It can be used with the
// NewPoller function to return a polling implementation of the Detector interface.
// This function returns true or false for leadership, or errors / exceptions that arise during the check.
// The consumer of this information will apply common criteria based on this raw data so that the implementation
// here won't have to do its own error handling or filtering and behavior can be enforced across all implementations.
type CheckLeaderFunc func() (bool, error)

// Leadership is a struct that captures the leadership state, possibly error if exception occurs
type Leadership struct {
	Status Status
	Error  error
}

// Detector is the interface for determining whether this instance is a leader
type Detector interface {

	// Start starts leadership detection
	Start() (<-chan Leadership, error)

	// Stop stops
	Stop()
}
