package fsm

import (
	"errors"

	log "github.com/golang/glog"
)

// ID is the id of the instance in a given set.  It's unique in that set.
type ID uint64

// Instance is the interface that returns ID and state of the fsm instance safely.
type Instance interface {
	ID() ID
	State() Index
	Signal(Signal) error
	CanReceive(Signal) bool
}

type instance struct {
	id       ID
	state    Index
	parent   *Set
	error    error
	flaps    flaps
	start    Time
	deadline Time
	index    int // index used in the deadlines queue
	visits   map[Index]int
}

// ID returns the ID of the fsm instance
func (i instance) ID() ID {
	return i.id
}

// State returns the state of the fsm instance
func (i instance) State() Index {
	result := make(chan Index)
	defer close(result)
	// we have to ask the set which actually holds the instance (this was returned by copy)
	i.parent.reads <- func(view Set) {
		if instance, has := view.members[i.id]; has {
			result <- instance.state
		}
	}
	return <-result
}

// Valid returns true if current state can receive the given signal
func (i instance) CanReceive(s Signal) bool {
	_, _, err := i.parent.spec.transition(i.State(), s)
	return err == nil
}

// Signal sends a signal to the instance
func (i instance) Signal(s Signal) (err error) {
	defer func() { log.V(100).Infoln("instance.signal: @id=", i.id, "signal=", s, "err=", err) }()

	if _, has := i.parent.spec.signals[s]; !has {
		err = unknownSignal(s)
		return
	}

	dest := i.parent.inputs
	if dest == nil {
		err = errors.New("not-initialized")
		return
	}

	dest <- &event{instance: i.id, signal: s}
	return nil
}

func (i *instance) update(next Index, now Time, ttl Tick) {
	i.visits[next] = i.visits[next] + 1
	i.state = next
	i.start = now
	if ttl > 0 {
		i.deadline = now + Time(ttl)
	} else {
		i.deadline = 0
	}
}
