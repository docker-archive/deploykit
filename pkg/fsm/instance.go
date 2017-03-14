package fsm

import (
	log "github.com/golang/glog"
)

// ID is the id of the instance in a given set.  It's unique in that set.
type ID uint64

// Instance is the interface that returns ID and state of the fsm instance safely.
type Instance interface {
	ID() ID
	State() (Index, bool)
	Signal(Signal) error
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
}

// ID returns the ID of the fsm instance
func (i instance) ID() ID {
	return i.id
}

// State returns the state of the fsm instance
func (i instance) State() (Index, bool) {
	result := make(chan Index)
	// we have to ask the set which actually holds the instance (this was returned by copy)
	i.parent.reads <- func(view Set) {
		if instance, has := view.members[i.id]; has {
			result <- instance.state
		}
		close(result)
	}
	v, ok := <-result
	return v, ok
}

// Signal sends a signal to the instance
func (i instance) Signal(s Signal) error {
	log.Infoln("Signaling", i.id, "signal=", s)

	dest, has := i.parent.inputs[s]
	if !has {
		return unknownSignal(s)
	}

	_, _, err := i.parent.spec.transition(i.state, s)
	if err != nil {
		return err
	}

	dest <- &event{instance: i.id, signal: s}
	return nil
}

func (i *instance) update(next Index, now Time, ttl Tick) {
	i.flaps.record(i.state, next)
	i.state = next
	i.start = now
	if ttl > 0 {
		i.deadline = now + Time(ttl)
	} else {
		i.deadline = 0
	}
}

func (i instance) raise(s Signal, inputs map[Signal]chan<- *event) {
	if ch, has := inputs[s]; has {
		ch <- &event{instance: i.id, signal: s}
	}
}
