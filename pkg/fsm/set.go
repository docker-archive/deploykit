package fsm

import (
	"time"
)

const (
	defaultBufferSize = 100
)

// NewSet returns a new set
func NewSet(spec *Spec, clock *Clock, optional ...Options) *Set {

	options := Options{}
	if len(optional) > 0 {
		options = optional[0]
	}

	if options.BufferSize == 0 {
		options.BufferSize = defaultBufferSize
	}

	set := &Set{
		options:      options,
		spec:         *spec,
		stop:         make(chan struct{}),
		clock:        clock,
		members:      map[ID]*instance{},
		bystate:      map[Index]map[ID]*instance{},
		reads:        make(chan func(Set)),
		add:          make(chan Index),
		delete:       make(chan ID),
		errors:       make(chan error),
		events:       make(chan *event),
		transactions: make(chan *txn, options.BufferSize),
		new:          make(chan Instance),
		deadlines:    newQueue(),
	}

	for i := range spec.states {
		set.bystate[i] = map[ID]*instance{}
	}

	set.run()
	set.running = true
	return set
}

// DefaultOptions returns default values
func DefaultOptions(name string) Options {
	return Options{
		Name:       name,
		BufferSize: defaultBufferSize,
	}
}

// Options contains options for the set
type Options struct {
	// Name is the name of the set
	Name string

	// BufferSize is the size of transaction queue/buffered channel
	BufferSize int
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
	add          chan Index // add an instance with initial state
	new          chan Instance
	delete       chan ID // delete an instance with id
	errors       chan error
	events       chan *event
	transactions chan *txn
	deadlines    *queue
	name         string
	running      bool
}

// Signal sends a signal to the instance
func (s *Set) Signal(signal Signal, instance ID, optionalData ...interface{}) error {
	if _, has := s.spec.signals[signal]; !has {
		return unknownSignal(signal)
	}

	var data interface{}
	if len(optionalData) > 0 {
		data = optionalData[0]
	}

	log.Info("Signal", "set", s.options.Name, "signal", signal, "instance", instance)
	s.events <- &event{instance: instance, signal: signal, data: data}
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
func (s *Set) ForEachInstance(view func(ID, Index, interface{}) bool) {
	blocker := make(chan struct{})
	s.reads <- func(set Set) {
		defer close(blocker)
		for _, m := range set.members {
			if view(m.id, m.state, m.data) {
				continue
			} else {
				break
			}
		}
	}
	<-blocker
}

// ForEachInstanceInState iterates through the set and provides a consistent view of the instances
func (s *Set) ForEachInstanceInState(check Index, view func(ID, Index, interface{}) bool) {
	blocker := make(chan struct{})
	s.reads <- func(set Set) {
		defer close(blocker)
		members, has := set.bystate[check]
		if !has {
			return
		}

		for _, m := range members {
			if view(m.id, m.state, m.data) {
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
	data     interface{}
}

func (s *Set) handleError(tid int64, err error, ctx interface{}) {
	log.Warn("error", "tid", tid, "err", err, "context", ctx)
	select {
	case s.errors <- err:
	default:
	}
}

func (s *Set) handleAdd(tid int64, initial Index) error {
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

	log.Debug("add: set deadline", "name", s.options.Name, "tid", tid, "id", id, "initial", initial)
	if err := s.processDeadline(tid, new, initial); err != nil {
		return err
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

func (s *Set) handleDelete(tid int64, id ID) error {
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

func (s *Set) tick() {
	s.now++
}

func (s *Set) ct() Time {
	return s.now
}

func (s *Set) handleClockTick(tid int64) error {

	s.tick()
	now := s.ct()

	log.Debug("Clock tick", "name", s.options.Name, "tid", tid, "now", now)

	for s.deadlines.Len() > 0 {

		instance := s.deadlines.peek()
		if instance == nil {
			return nil
		}

		if instance.deadline > now {
			return nil
		}

		instance = s.deadlines.dequeue()

		// check > 0 here because we could have already raised the signal
		// when a real event came in.
		if instance.deadline > 0 {
			// raise the signal
			if ttl, err := s.spec.expiry(instance.state); err != nil {

				return err

			} else if ttl != nil {

				log.Warn("deadline exceeded", "name", s.options.Name, "tid", tid, "id", instance.id, "raise", ttl.Raise)

				s.raise(tid, instance.id, ttl.Raise, instance.state)
			}
		}
		// reset the state for future queueing
		instance.deadline = -1
		instance.index = -1

	}
	return nil
}

func (s *Set) processDeadline(tid int64, instance *instance, state Index) error {
	now := s.ct()
	ttl := Tick(0)
	// check for TTL
	if exp, err := s.spec.expiry(state); err != nil {
		return err
	} else if exp != nil {
		ttl = exp.TTL
	}

	instance.update(state, now, ttl)

	if instance.index > -1 {
		// case where this instance is in the deadlines queue (since it has a > -1 index)
		if instance.deadline > 0 {
			// in the queue and deadline is different now
			log.Debug("deadline updating", "name", s.options.Name, "tid", tid,
				"instance", instance.id, "deadline", instance.deadline,
				"state", instance.index)
			s.deadlines.update(instance)
		} else {
			log.Debug("deadline removing", "name", s.options.Name, "tid", tid,
				"instance", instance.id, "deadline", instance.deadline,
				"state", instance.index)
			s.deadlines.remove(instance)
		}
	} else if instance.deadline > 0 {
		// index == -1 means it's not in the queue yet and we have a deadline
		log.Debug("deadline enqueuing", "name", s.options.Name, "tid", tid,
			"instance", instance.id, "deadline", instance.deadline, "state", instance.index)
		s.deadlines.enqueue(instance)
	}

	return nil
}

func (s *Set) processVisitLimit(tid int64, instance *instance, state Index) error {
	// have we visited next state too many times?
	if limit, err := s.spec.visit(state); err != nil {

		return err

	} else if limit != nil {

		if limit.Value > 0 && instance.visits[state] == limit.Value {

			log.Debug("Max visit limit hit", "name", s.options.Name, "tid", tid,
				"instance", instance.id, "state", instance.state, "raise", limit.Raise)

			s.raise(tid, instance.id, limit.Raise, instance.state)

			return nil
		}
	}
	return nil
}

// raises a signal by placing directly on the txn queue
func (s *Set) raise(tid int64, id ID, signal Signal, current Index) (err error) {
	defer func() {
		log.Debug("instance.signal", "name", s.options.Name, "instance", id,
			"signal", signal, "state", current, "err", err)
	}()

	if _, has := s.spec.signals[signal]; !has {
		err = unknownSignal(signal)
		return
	}

	event := &event{instance: id, signal: signal}

	s.transactions <- &txn{
		Func: func(tid int64) (interface{}, error) {
			return event, s.handleEvent(tid, event)
		},
		tid: tid,
	}
	return nil
}

func (s *Set) handleEvent(tid int64, event *event) error {

	instance, has := s.members[event.instance]
	if !has {
		return unknownInstance(event.instance)
	}

	current := instance.state
	next, action, err := s.spec.transition(current, event.signal)
	if err != nil {
		return err
	}

	log.Debug("Transition", "name", s.options.Name, "tid", tid,
		"instance", instance.id, "state", current, "signal", event.signal, "next", next,
		"deadline", instance.deadline, "index", instance.index)

	// any flap detection?
	limit := s.spec.flap(current, next)
	if limit != nil && limit.Count > 0 {

		instance.flaps.record(current, next)
		flaps := instance.flaps.count(current, next)

		if flaps >= limit.Count {

			log.Warn("Flapping", "tid", tid, "flaps", flaps,
				"instance", instance.id, "state", instance.state, "raise", limit.Raise)
			s.raise(tid, instance.id, limit.Raise, instance.state)

			return nil // done -- another transition
		}
	}

	// Associate custom data - do this before calling on the action so action can do something with it.
	if event.data != nil {
		instance.data = event.data
	}

	log.Debug("Transtion", "action", action)
	// call action before transitiion
	if action != nil {
		if err := action(instance); err != nil {

			log.Warn("Error transition", "err", err)

			if alternate, err := s.spec.error(current, event.signal); err != nil {

				s.handleError(tid, err, []interface{}{current, event, instance})

			} else {

				log.Warn("Err executing action", "tid", tid, "instance", instance.id,
					"state", current, "signal", event.signal, "alternate", alternate, "next", next)

				next = alternate
			}
		}
	}

	// process deadline, if any
	if err := s.processDeadline(tid, instance, next); err != nil {
		return err
	}

	// update the index
	delete(s.bystate[current], instance.id)
	s.bystate[next][instance.id] = instance

	// visits limit trigger
	return s.processVisitLimit(tid, instance, next)
}

func (s *Set) tid() int64 {
	return time.Now().UnixNano()
}

type txn struct {
	Func func(int64) (interface{}, error)
	tid  int64
}

func (s *Set) run() {

	stopTransactions := make(chan struct{})
	// Core processing
	go func() {
		defer func() {
			log.Info("Shutting down", "name", s.options.Name)
			close(s.transactions)
		}()

		for {
			select {
			case <-stopTransactions:
				return

			case t := <-s.transactions:
				if t == nil {
					return
				}
				if ctx, err := t.Func(t.tid); err != nil {
					s.handleError(t.tid, err, ctx)
				}

			}
		}
	}()

	stopTimer := make(chan struct{})
	// timer
	go func() {
		for {
			select {
			case <-stopTimer:
				return

			case <-s.clock.C:
				s.transactions <- &txn{
					tid: s.tid(),
					Func: func(tid int64) (interface{}, error) {
						return nil, s.handleClockTick(tid)
					},
				}
			}
		}
	}()

	// Input events
	go func() {

	loop:
		for {

			var tx *txn
			tid := s.tid()

			select {

			case <-s.stop:
				close(stopTimer)
				break loop

			case initial, ok := <-s.add:
				// add new instance
				if !ok {
					break loop
				}

				copy := initial
				tx = &txn{
					tid: tid,
					Func: func(tid int64) (interface{}, error) {
						return copy, s.handleAdd(tid, copy)
					},
				}

			case id, ok := <-s.delete:
				// delete instance
				if !ok {
					break loop
				}

				copy := id
				tx = &txn{
					tid: tid,
					Func: func(tid int64) (interface{}, error) {
						return copy, s.handleDelete(tid, copy)
					},
				}

			case event, ok := <-s.events:
				// state transition events
				if !ok {
					break loop
				}

				copy := event
				tx = &txn{
					tid: tid,
					Func: func(tid int64) (interface{}, error) {
						return copy, s.handleEvent(tid, copy)
					},
				}

			case reader := <-s.reads:
				tx = &txn{
					tid: tid,
					Func: func(tid int64) (interface{}, error) {
						// For reads on the Set itself.  All the reads are serialized.
						view := *s // a copy (not quite deep copy) of the set
						reader(view)
						return nil, nil
					},
				}
			}

			// send to transaction processing pipeline
			s.transactions <- tx

		}

	}()
}
