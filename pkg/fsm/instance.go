package fsm

import (
	"sync"
)

// ID is the id of the instance in a given set.  It's unique in that set.
type ID uint64

// Instance is the interface that returns ID and state of the fsm instance safely.
type Instance interface {

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

type instance struct {
	id       ID
	state    Index
	data     interface{}
	parent   *Set
	error    error
	flaps    flaps
	start    Time
	deadline Time
	index    int // index used in the deadlines queue
	visits   map[Index]int

	lock sync.RWMutex
}

// ID returns the ID of the fsm instance
func (i *instance) ID() ID {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.id
}

// Data returns a customer data value attached to this instance
func (i *instance) Data() interface{} {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.data
}

const invalidState Index = -99999

// IsInvalidState returns true if the index is invalid
func IsInvalidState(s Index) bool {
	return s == invalidState
}

// State returns the state of the fsm instance
func (i *instance) State() (result Index) {
	done := make(chan struct{})

	result = invalidState

	// queue this so that the read is consistent
	i.parent.reads <- func(view Set) {
		defer close(done)

		instance, has := view.members[i.id]
		if has {
			result = instance.state
		}
		i.lock.Lock()
		defer i.lock.Unlock()
		i.state = result // update self
	}
	<-done // finish waiting
	return
}

// Valid returns true if current state can receive the given signal
func (i *instance) CanReceive(s Signal) bool {
	_, _, err := i.parent.spec.transition(i.State(), s)
	return err == nil
}

// Signal sends a signal to the instance
func (i *instance) Signal(s Signal, optionalData ...interface{}) (err error) {
	return i.parent.Signal(s, i.id, optionalData...)
}

func (i *instance) update(next Index, now Time, ttl Tick) {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.visits[next] = i.visits[next] + 1
	i.state = next
	i.start = now
	if ttl > 0 {
		i.deadline = now + Time(ttl)
	} else {
		i.deadline = 0
	}
}
