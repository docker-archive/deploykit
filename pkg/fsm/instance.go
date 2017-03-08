package fsm

type Instance interface {
	ID() uint64
	State() (Index, bool)
}

type instance struct {
	id       id
	state    Index
	parent   *Set
	error    error
	flaps    flaps
	start    Time
	deadline Time
	index    int // index used in the deadlines queue
}

func (i instance) ID() uint64 {
	return uint64(i.id)
}

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

func (i instance) Signal(s Signal) error {
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
