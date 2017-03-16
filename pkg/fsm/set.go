package fsm

import (
	log "github.com/golang/glog"
)

// NewSet returns a new set
func NewSet(spec *Spec, clock *Clock) *Set {
	set := &Set{
		spec:      *spec,
		stop:      make(chan struct{}),
		clock:     clock,
		members:   map[ID]*instance{},
		bystate:   map[Index]map[ID]*instance{},
		reads:     make(chan func(Set)),
		add:       make(chan Index),
		delete:    make(chan ID),
		errors:    make(chan error),
		new:       make(chan Instance),
		deadlines: newQueue(),
	}

	for i := range spec.states {
		set.bystate[i] = map[ID]*instance{}
	}

	set.inputs = set.run()
	set.running = true
	return set
}

// Set is a collection of fsm instances that follow a given spec.  This is
// the primary interface to manipulate the instances... by sending signals to it via channels.
type Set struct {
	spec      Spec
	now       Time
	next      ID
	clock     *Clock
	members   map[ID]*instance
	bystate   map[Index]map[ID]*instance
	reads     chan func(Set) // given a view which is a copy of the Set
	stop      chan struct{}
	add       chan Index // add an instance with initial state
	new       chan Instance
	delete    chan ID // delete an instance with id
	errors    chan error
	inputs    chan<- *event
	deadlines *queue

	running bool
}

// Signal sends a signal to the instance
func (s *Set) Signal(signal Signal, instance ID) error {
	if _, has := s.spec.signals[signal]; !has {
		return unknownSignal(signal)
	}

	log.V(100).Infoln("signal", signal, "to instance=", instance)
	s.inputs <- &event{instance: instance, signal: signal}
	return nil
}

// Size returns the size of the set
func (s *Set) Size() int {
	result := make(chan int, 1)
	defer close(result)
	s.reads <- func(view Set) {
		result <- len(view.members)
	}
	return <-result
}

// CountByState returns a count of instances in a given state.
func (s *Set) CountByState(state Index) int {
	total := make(chan int, 1)
	s.reads <- func(set Set) {
		defer close(total)
		total <- len(s.bystate[state])
	}
	return <-total
}

// ForEachInstance iterates through the set and provides a consistent view of the instances
func (s *Set) ForEachInstance(view func(ID, Index) bool) {
	blocker := make(chan struct{})
	s.reads <- func(set Set) {
		defer close(blocker)
		for _, m := range set.members {
			if view(m.id, m.state) {
				continue
			} else {
				break
			}
		}
	}
	<-blocker
}

// Instance returns the instance by id
func (s *Set) Instance(id ID) Instance {
	blocker := make(chan Instance, 1)
	s.reads <- func(set Set) {
		defer close(blocker)
		blocker <- set.members[id]
	}
	return <-blocker
}

// Add adds an instance given initial state
func (s *Set) Add(initial Index) Instance {
	s.add <- initial
	return <-s.new
}

// Delete deletes an instance
func (s *Set) Delete(instance Instance) {
	s.delete <- instance.ID()
}

// Stop stops the state machine loop
func (s *Set) Stop() {
	if s.running {
		close(s.stop)
		s.clock.Stop()
		s.running = false
	}
}

type event struct {
	instance ID
	signal   Signal
}

func (s *Set) handleError(err error, ctx interface{}) {
	log.Warningln("error occurred:", err, "context=", ctx)
	select {
	case s.errors <- err:
	default:
	}
}

func (s *Set) handleAdd(initial Index) error {
	// add a new instance
	id := s.next
	s.next++

	new := &instance{
		id:     id,
		state:  initial,
		index:  -1,
		parent: s,
		flaps:  *newFlaps(),
		visits: map[Index]int{
			initial: 1,
		},
	}

	log.V(100).Infoln("add:", "id=", id, "initial=", initial, "set deadline.")
	if err := s.processDeadline(new, initial); err != nil {
		s.handleError(err, new)
	}

	// update index
	s.members[id] = new
	s.bystate[initial][id] = new

	// return a copy here so we don't have problems with races trying to read / write the same pointer
	s.new <- &instance{
		id:     new.id,
		parent: s,
	}

	return nil
}

func (s *Set) handleDelete(id ID) error {
	instance, has := s.members[id]
	if !has {
		return unknownInstance(id)
	}
	// delete an instance and update index
	delete(s.members, id)
	delete(s.bystate[instance.state], id)

	// for safety
	instance.id = ID(0)
	instance.parent = nil

	return nil
}

func (s *Set) handleClockTick(inputs chan<- *event) error {
	defer func(start Time) {
		log.V(100).Infoln("CLOCK (", start, ",", s.now, ") ========================================================")
	}(s.now)

	s.now++

	// go through the priority queue by deadline and raise signals if expired.
	instance := s.deadlines.peek()
	if instance == nil {
		return nil
	}

	if instance.deadline > s.now {
		return nil
	}

	for s.deadlines.Len() > 0 {

		instance = s.deadlines.dequeue()

		log.Infoln("t=", s.now, "id=", instance.id, "deadline=", instance.deadline)

		if instance.deadline == s.now {

			// check > 0 here because we could have already raised the signal
			// when a real event came in.
			if instance.deadline > 0 {
				// raise the signal
				if ttl, err := s.spec.expiry(instance.state); err != nil {

					return err

				} else if ttl != nil {

					log.V(100).Infoln("deadline exceeded:", "@id=", instance.id, "raise=", ttl.Raise)

					instance.Signal(ttl.Raise)
				}
			}
			// reset the state for future queueing
			instance.deadline = -1
			instance.index = -1

		}

		instance = s.deadlines.peek()
		if instance == nil {
			break
		}

		if instance.deadline > s.now {
			break
		}
	}
	return nil
}

func (s *Set) processDeadline(instance *instance, state Index) error {
	ttl := Tick(0)
	// check for TTL
	if exp, err := s.spec.expiry(state); err != nil {
		return err
	} else if exp != nil {
		ttl = exp.TTL
	}

	instance.update(state, s.now, ttl)

	log.V(100).Infoln("deadline: deadline=", instance.deadline, "priority=", instance.index)

	if instance.index > -1 {
		// case where this instance is in the deadlines queue (since it has a > -1 index)
		if instance.deadline > 0 {
			// in the queue and deadline is different now
			log.V(100).Infoln("deadline: updating instance", instance.id, "deadline=", instance.deadline, "at", instance.index)
			s.deadlines.update(instance)
		} else {
			log.V(100).Infoln("deadline: removing instance", instance.id, "deadline=", instance.deadline, "at", instance.index)
			s.deadlines.remove(instance)
		}
	} else if instance.deadline > 0 {
		// index == -1 means it's not in the queue yet and we have a deadline
		log.V(100).Infoln("deadline: enqueuing instance", instance.id, "deadline=", instance.deadline, "at", instance.index)
		s.deadlines.enqueue(instance)
	}

	return nil
}

func (s *Set) processVisitLimit(instance *instance, state Index) error {
	// have we visited next state too many times?
	if limit, err := s.spec.visit(state); err != nil {

		return err

	} else if limit != nil {

		log.V(100).Infoln("found limit:", limit.Value, "target=", state, "visits=", instance.visits[state])

		if limit.Value > 0 && instance.visits[state] == limit.Value {

			log.Warningln("max visits hit, raising:", instance.state, "=[", limit.Raise, "]=>")
			instance.Signal(limit.Raise)

			return nil
		}
	}
	return nil
}

func (s *Set) handleEvent(event *event, inputs chan<- *event) error {

	instance, has := s.members[event.instance]
	if !has {
		return unknownInstance(event.instance)
	}

	current := instance.state
	next, action, err := s.spec.transition(current, event.signal)
	if err != nil {
		return err
	}

	log.V(100).Infoln("transition @id=", instance.id, "::::", "[", current, "]--(", event.signal, ")-->", "[", next, "]",
		"deadline=", instance.deadline, "index=", instance.index)

	// any flap detection?
	limit := s.spec.flap(current, next)
	if limit != nil && limit.Count > 0 {

		instance.flaps.record(current, next)
		flaps := instance.flaps.count(current, next)

		if flaps >= limit.Count {

			log.Warningln("flap detected, raising", limit.Raise)
			instance.Signal(limit.Raise)

			return nil // done -- another transition
		}
	}

	// call action before transitiion
	if action != nil {
		if err := action(instance); err != nil {

			if rule, err := s.spec.error(current, event.signal); err != nil {

				s.handleError(err, []interface{}{current, event, instance})

			} else if rule != nil {

				log.Warningln("error executing action, raising", rule.Raise)
				instance.Signal(rule.Raise)

				return nil // done - another transition
			}
		}
	}

	// process deadline, if any
	if err := s.processDeadline(instance, next); err != nil {
		return err
	}

	// update the index
	delete(s.bystate[current], instance.id)
	s.bystate[next][instance.id] = instance

	// visits limit trigger
	if err := s.processVisitLimit(instance, next); err != nil {
		return err
	}

	return nil
}

func (s *Set) run() chan<- *event {

	events := make(chan *event)
	transactions := make(chan func() (interface{}, error), BufferedChannelSize)

	// Core processing
	go func() {
		defer func() {
			log.Infoln("set shutting down.")
		}()

		for {
			txn, open := <-transactions
			if !open {
				return
			}

			if ctx, err := txn(); err != nil {
				s.handleError(err, ctx)
			}

		}
	}()

	// Input events
	go func() {

	loop:
		for {

			tx := func() (interface{}, error) { return nil, nil } // no-op

			select {

			case <-s.stop:
				break loop

			case <-s.clock.C:

				log.V(100).Infoln("CLOCK")

				tx = func() (interface{}, error) {
					return nil, s.handleClockTick(events)
				}

			case initial, ok := <-s.add:
				// add new instance
				if !ok {
					break loop
				}

				copy := initial
				tx = func() (interface{}, error) {
					return copy, s.handleAdd(copy)
				}

			case id, ok := <-s.delete:
				// delete instance
				if !ok {
					break loop
				}

				copy := id
				tx = func() (interface{}, error) {
					return copy, s.handleDelete(copy)
				}

			case event, ok := <-events:
				// state transition events
				if !ok {
					break loop
				}

				copy := event
				tx = func() (interface{}, error) {
					return copy, s.handleEvent(copy, events)
				}

			case reader := <-s.reads:
				tx = func() (interface{}, error) {
					// For reads on the Set itself.  All the reads are serialized.
					view := *s // a copy (not quite deep copy) of the set
					reader(view)
					return nil, nil
				}

			}

			// send to transaction processing pipeline
			transactions <- tx

		}
	}()

	return events
}
