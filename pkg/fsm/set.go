package fsm

import (
	"fmt"
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
		add:          make(chan addOp),
		delete:       make(chan ID),
		errors:       make(chan error),
		events:       make(chan *event),
		transactions: make(chan *txn, options.BufferSize),
		deadlines:    newQueue(),
	}

	for i := range spec.states {
		set.bystate[i] = map[ID]*instance{}
	}

	set.run()
	set.running = true
	return set
}

// Signal sends a signal to the instance
func (s *Set) Signal(signal Signal, instance ID, optionalData ...interface{}) error {
	if _, has := s.spec.signals[signal]; !has {
		return ErrUnknownSignal{Signal: signal}
	}

	log.Debug("Signal", "set", s.options.Name, "signal", s.spec.SignalName(signal), "instance", instance)
	s.events <- &event{instance: instance, signal: signal, data: optionalData}
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

// ForEach iterates through the set and provides a consistent view of the instances
func (s *Set) ForEach(view func(ID, Index, interface{}) bool) {
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

// ForEachInState iterates through the set and provides a consistent view of the instances
func (s *Set) ForEachInState(check Index, view func(ID, Index, interface{}) bool) {
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

// Name returns the name of the set
func (s *Set) Name() string {
	return s.options.Name
}

// Get returns the instance by id. Nil if no id matched
func (s *Set) Get(id ID) (found FSM) {
	blocker := make(chan struct{})
	s.reads <- func(set Set) {
		defer close(blocker)
		found = set.members[id]
	}
	<-blocker
	return
}

// Add adds an instance given initial state
func (s *Set) Add(initial Index) FSM {
	op := addOp{initial: initial, result: make(chan FSM)}
	s.add <- op
	return <-op.result
}

// Delete deletes an instance
func (s *Set) Delete(instance FSM) {
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

// Errors returns the errors encountered during async processing of events
func (s *Set) Errors() <-chan error {
	return s.errors
}

type event struct {
	instance ID
	signal   Signal
	data     []interface{}
}

func (s *Set) handleError(tid int64, err error, ctx interface{}) {

	message := err.Error()
	switch err := err.(type) {
	case ErrUnknownState:
		if s.options.IgnoreUndefinedStates {
			return
		}
		message = fmt.Sprintf("%s: %v", err.Error(),
			s.spec.StateName(Index(err)))

	case ErrUnknownTransition:
		if s.options.IgnoreUndefinedTransitions {
			return
		}
		message = fmt.Sprintf("%s: state(%v) on signal(%v)", err.Error(),
			s.spec.StateName(err.State), s.spec.SignalName(err.Signal))

	case ErrUnknownSignal:
		if s.options.IgnoreUndefinedSignals {
			return
		}
		message = fmt.Sprintf("%s: state(%v) on signal(%v)", err.Error(),
			s.spec.StateName(Index(err.State)), s.spec.SignalName(Signal(err.Signal)))

	case ErrDuplicateState:
		message = fmt.Sprintf("%s: %v", err.Error(),
			s.spec.StateName(Index(err)))

	case ErrUnknownFSM:
		message = fmt.Sprintf("%s: %v", err.Error(), err)
	}

	defer log.Error("error", "tid", tid, "err", message, "context", ctx)
	select {
	case s.errors <- err: // non-blocking send
	default:
	}
}
func (s *Set) handleAdd(tid int64, op addOp) error {
	// add a new instance
	id := s.next
	s.next++

	new := &instance{
		id:     id,
		state:  op.initial,
		index:  -1,
		parent: s,
		flaps:  *newFlaps(),
		visits: map[Index]int{
			op.initial: 1,
		},
	}

	if err := s.processDeadline(tid, new, op.initial); err != nil {
		log.Error("error process deadline", "err", err)
		return err
	}
	if new.index > -1 {
		log.Debug("Set deadline", "name", s.options.Name,
			"tid", tid, "id", id, "initial", s.spec.StateName(op.initial),
			"deadline", new.deadline, "queuePosition", new.index)
	}
	// update index
	s.members[id] = new
	s.bystate[op.initial][id] = new

	// return a copy here so we don't have problems with races trying to read / write the same pointer
	op.result <- &instance{
		id:     new.id,
		parent: s,
	}
	return nil
}

func (s *Set) handleDelete(tid int64, id ID) error {
	instance, has := s.members[id]
	if !has {
		return ErrUnknownFSM(id)
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

	log.Debug("Clock tick", "name", s.options.Name, "tid", tid, "now", now, "V", debugV2)

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

				log.Error("deadline exceeded", "name", s.options.Name, "tid", tid, "id", instance.id,
					"raise", s.spec.SignalName(ttl.Raise), "now", now)

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
			log.Debug("Deadline updating", "now", now, "name", s.options.Name, "tid", tid,
				"instance", instance.id, "deadline", instance.deadline,
				"deadline-queue-index", instance.index, "V", debugV2)
			s.deadlines.update(instance)
		} else {
			log.Debug("Deadline removing", "now", now, "name", s.options.Name, "tid", tid,
				"instance", instance.id, "deadline", instance.deadline,
				"deadline-queue-index", instance.index, "V", debugV2)
			s.deadlines.remove(instance)
		}
	} else if instance.deadline > 0 {
		// index == -1 means it's not in the queue yet and we have a deadline
		log.Debug("Deadline enqueuing", "now", now, "name", s.options.Name, "tid", tid,
			"instance", instance.id, "deadline", instance.deadline,
			"deadline-queue-index", instance.index, "V", debugV2)
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
				"instance", instance.id, "state", s.spec.StateName(instance.state),
				"raise", s.spec.SignalName(limit.Raise))

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
			"signal", s.spec.SignalName(signal), "state", s.spec.StateName(current), "err", err)
	}()

	if _, has := s.spec.signals[signal]; !has {
		err = ErrUnknownSignal{Signal: signal}
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

	now := s.ct()

	instance, has := s.members[event.instance]
	if !has {
		return ErrUnknownFSM(event.instance)
	}

	current := instance.state
	next, action, err := s.spec.transition(current, event.signal)
	if err != nil {
		return err
	}

	log.Debug("Transition",
		"now", now,
		"name", s.options.Name, "tid", tid,
		"instance", instance.id,
		"state", s.spec.StateName(current),
		"signal", s.spec.SignalName(event.signal),
		"next", s.spec.StateName(next),
		"deadline", instance.deadline, "deadlineQueueIndex", instance.index)

	// any flap detection?
	limit := s.spec.flap(current, next)
	if limit != nil && limit.Count > 0 {

		instance.flaps.record(current, next)
		flaps := instance.flaps.count(current, next)

		if flaps >= limit.Count {

			log.Debug("Flapping", "tid", tid, "flaps", flaps,
				"instance", instance.id, "state", instance.state, "raise", limit.Raise)
			s.raise(tid, instance.id, limit.Raise, instance.state)

			return nil // done -- another transition
		}
	}

	// Associate custom data - do this before calling on the action so action can do something with it.
	if event.data != nil {
		instance.data = event.data
	}

	// call action before transitiion
	if action != nil {

		log.Debug("Invoking action",
			"now", now,
			"name", s.options.Name, "tid", tid,
			"instance", instance.id,
			"state", s.spec.StateName(current),
			"signal", s.spec.SignalName(event.signal),
			"next", s.spec.StateName(next),
			"deadline", instance.deadline, "deadlineQueueIndex", instance.index)

		if err := action(instance); err != nil {

			log.Debug("Error transition", "err", err)

			if alternate, err := s.spec.error(current, event.signal); err != nil {

				s.handleError(tid, err, []interface{}{current, event, instance})

			} else {

				log.Debug("Err executing action", "tid", tid, "instance", instance.id,
					"state", current, "signal", event.signal, "alternate", alternate, "next", next)

				next = alternate
			}
		}
	}

	// Action has been run... We landed in the new state (next)

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

	// Input events

	go func() {

	loop:
		for {

			var tx *txn
			tid := s.tid()

			select {

			case <-s.clock.C:
				tx = &txn{
					tid: s.tid(),
					Func: func(tid int64) (interface{}, error) {
						return nil, s.handleClockTick(tid)
					},
				}

			case <-s.stop:
				break loop

			case initial, ok := <-s.add:
				// add new instance
				if !ok {
					break loop
				}
				tx = &txn{
					tid: tid,
					Func: func(tid int64) (interface{}, error) {
						return initial.initial, s.handleAdd(tid, initial)
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
						reader(*s)
						return nil, nil
					},
				}
			}

			// send to transaction processing pipeline
			s.transactions <- tx

		}

	}()
}
