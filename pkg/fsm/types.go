package fsm

// ID is the id of the instance in a given set.  It's unique in that set.
type ID uint64

// FSM is the interface that returns ID and state of the fsm instance safely.
type FSM interface {

	// ID returns the ID of the instance
	ID() ID

	// State returns the state of the instance. This is an expensive call to be submitted to queue to view
	State() Index

	// Data returns the custom data attached to the instance.  It's set via the optional arg in Signal
	Data() interface{}

	// Signal signals the instance with optional custom data
	Signal(Signal, ...interface{}) error

	// CanReceive returns true if the current state of the instance can receive the given signal
	CanReceive(Signal) bool
}

// Index is the index of the state in a FSM
type Index int

// Action is the action to take when a signal is received, prior to transition
// to the next state.  The error returned by the function is an exception which
// will put the state machine in an error state.  This error state is not the same
// as some application-specific error state which is a state defined to correspond
// to some external event indicating a real-world error event (as opposed to a
// programming error here).
type Action func(FSM) error

// Tick is a unit of time. Time is in relative terms and synchronized with an actual
// timer that's provided by the client.
type Tick int64

// Time is a unit of time not corresponding to wall time
type Time int64

// Expiry specifies the rule for TTL..  A state can have TTL / deadline that when it
// expires a signal can be raised.
type Expiry struct {
	TTL   Tick
	Raise Signal
}

// Limit is a struct that captures the limit and what signal to raise
type Limit struct {
	Value int
	Raise Signal
}

// Signal is a signal that can drive the state machine to transfer from one state to next.
type Signal int

// State encapsulates all the possible transitions and actions to perform during the
// state transition.  A state can have a TTL so that it is allowed to be in that
// state for a given TTL.  On expiration, a signal is raised.
type State struct {

	// Index is a unique key of the state
	Index Index

	// Transitions fully specifies all the possible transitions from this state, by the way of signals.
	Transitions map[Signal]Index

	// Actions specify for each signal, what code / action is to be executed as the fsm transits from one state to next.
	Actions map[Signal]Action

	// Errors specifies the handling of errors when executing action.  On action error, the mapped state is transitioned.
	Errors map[Signal]Index

	// TTL specifies how long this state can last before a signal is raised.
	TTL Expiry

	// Visit specifies a limit on the number of times the fsm can visit this state before raising a signal.
	Visit Limit
}

// DefaultOptions returns default values
func DefaultOptions(name string) Options {
	return Options{
		Name:                       name,
		BufferSize:                 defaultBufferSize,
		IgnoreUndefinedTransitions: true,
		IgnoreUndefinedSignals:     true,
		IgnoreUndefinedStates:      true,
	}
}

// Options contains options for the set
type Options struct {
	// Name is the name of the set
	Name string

	// BufferSize is the size of transaction queue/buffered channel
	BufferSize int

	// IgnoreUndefinedStates will not report error from undefined states for transition on Error() chan, if true
	IgnoreUndefinedStates bool

	// IgnoreUndefinedTransitions will not report error from undefined transitions for signal on Error() chan, if true
	IgnoreUndefinedTransitions bool

	// IgnoreUndefinedSignals will not report error from undefined signal for the state on Error() chan, if true
	IgnoreUndefinedSignals bool
}

type addOp struct {
	initial Index
	result  chan FSM
}

// Set is a collection of fsm instances that follow a given spec.  This is
// the primary interface to manipulate the instances... by sending signals to it via channels.
type Set struct {
	options      Options
	spec         Spec
	now          Time
	next         ID
	clock        *Clock
	members      map[ID]*instance
	bystate      map[Index]map[ID]*instance
	reads        chan func(Set) // given a view which is a copy of the Set
	stop         chan struct{}
	add          chan addOp
	delete       chan ID // delete an instance with id
	errors       chan error
	events       chan *event
	transactions chan *txn
	deadlines    *queue
	running      bool
}
