package fsm

import (
	"time"
)

// NewSet returns a new set
func NewSet(spec *Spec, tick <-chan time.Time) *Set {
	set := &Set{
		spec:    *spec,
		stop:    make(chan struct{}),
		members: map[id]*instance{},
		reads:   make(chan func(Set)),
		add:     make(chan Index),
		delete:  make(chan id),
		errors:  make(chan error),
		new:     make(chan Instance),
	}
	if tick != nil {
		go set.run(tick)
	}
	return set
}

// Set is a collection of fsm instances that follow a given spec.  This is
// the primary interface to manipulate the instances... by sending signals to it via channels.
type Set struct {
	spec    Spec
	now     Time
	next    id
	members map[id]*instance
	reads   chan func(Set) // given a view which is a copy of the Set
	stop    chan struct{}
	add     chan Index // add an instance with initial state
	new     chan Instance
	delete  chan id // delete an instance with id
	errors  chan error
}

func (s *Set) Size() int {
	result := make(chan int)
	s.reads <- func(view Set) {
		result <- len(view.members)
	}
	return <-result
}

func (s *Set) Add(initial Index) Instance {
	s.add <- initial
	return <-s.new
}

func (s *Set) Delete(instance Instance) {
	s.delete <- id(instance.ID())
}

func (s *Set) Stop() {
	if s.stop != nil {
		close(s.stop)
	}
}

type id uint64
type event struct {
	instance id
	signal   Signal
}

func (s *Set) run(tick <-chan time.Time) map[Signal]chan<- *event {

	// Start up the goroutines to merge all the events/triggers.
	// Note we use merge channel for performance (over the slower reflect.Select) and for readability.
	collector := make(chan *event)
	inputs := map[Signal]chan<- *event{}
	for _, signal := range s.spec.signals {
		input := make(chan *event)
		inputs[signal] = input
		go func() {
			for {
				event, ok := <-input
				if !ok {
					return
				}
				collector <- event
			}
		}()
	}

loop:
	for {
		select {

		case <-s.stop:
			break loop

		case <-tick:

			// update the 'clock'
			s.now++

			// go through a list of instances that have deadline and raise signals if expired

		case initial := <-s.add:

			// add a new instance
			id := s.next
			s.next++

			new := &instance{
				id:     id,
				state:  initial,
				parent: s,
				flaps:  *newFlaps(),
			}

			s.members[id] = new
			s.new <- new

		case id := <-s.delete:

			// delete an instance
			delete(s.members, id)

		case event, ok := <-collector:

			// state transition events
			if !ok {
				break loop
			}

			// process events here.
			if instance, has := s.members[event.instance]; has {

				// check for TTL
				if instance.expired(s.now) {

					// raise the signal instead of continuing
					if ttl, ok := s.spec.expiry(instance.state); ok {
						instance.raise(ttl.Raise, inputs)
					}

					continue loop
				}

				current := instance.state
				next, action, err := s.spec.transition(current, event.signal)
				if err != nil {
					select {
					case s.errors <- err:
					default:
					}
				}

				// any flap detection?
				limit := s.spec.flap(current, next)
				if limit.Count > 0 {
					if instance.flaps.count(current, next) > limit.Count {
						instance.raise(limit.Raise, inputs)

						continue loop
					}
				}

				// call action before transitiion
				action()

				ttl := Tick(0)
				// check for TTL
				if exp, ok := s.spec.expiry(next); ok {
					ttl = exp.TTL
				}

				instance.update(next, s.now, ttl)
			}

		case reader := <-s.reads:
			// For reads on the Set itself.  All the reads are serialized.
			view := *s // a copy (not quite deep copy) of the set
			reader(view)
		}
	}

	return inputs
}

type Instance interface {
	ID() uint64
}

type instance struct {
	id     id
	state  Index
	parent *Set
	error  error
	flaps  flaps
	start  Time
	ttl    Tick
}

func (i instance) ID() uint64 {
	return uint64(i.id)
}

func (i instance) expired(now Time) bool {
	return Tick(now-i.start) >= i.ttl
}

func (i instance) Signal(s Signal) error {
	return nil
}

func (i *instance) update(next Index, now Time, ttl Tick) {
	i.flaps.record(i.state, next)
	i.state = next
	i.start = now
	i.ttl = ttl
}

func (i instance) raise(s Signal, inputs map[Signal]chan<- *event) {
	if ch, has := inputs[s]; has {
		ch <- &event{instance: i.id, signal: s}
	}
}
