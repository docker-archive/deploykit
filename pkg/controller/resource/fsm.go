package resource

import (
	"sync"
	"time"

	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/fsm"
)

// Model encapsulates the workflow / state machines for provisioning resources
type Model struct {
	spec     *fsm.Spec
	set      *fsm.Set
	clock    *fsm.Clock
	tickSize time.Duration

	resource.Properties

	instanceDestroyChan   chan fsm.FSM
	instanceProvisionChan chan fsm.FSM
	instancePendingChan   chan fsm.FSM
	instanceReadyChan     chan fsm.FSM
	cleanupChan           chan fsm.FSM

	lock sync.RWMutex
}

// Cleanup is the channel to get signals to clean up
func (m *Model) Cleanup() <-chan fsm.FSM {
	return m.cleanupChan
}

// Destroy is the channel to get signals to destroy an instance
func (m *Model) Destroy() <-chan fsm.FSM {
	return m.instanceDestroyChan
}

// Pending is the channel to get signals that instances are in pending state
func (m *Model) Pending() <-chan fsm.FSM {
	return m.instancePendingChan
}

// Ready is the channel to get signals of instances that are ready
func (m *Model) Ready() <-chan fsm.FSM {
	return m.instanceReadyChan
}

// Provision is the channel to get signals to provision new instance
func (m *Model) Provision() <-chan fsm.FSM {
	return m.instanceProvisionChan
}

// Requested adds a new fsm in the requested state
func (m *Model) Requested() fsm.FSM {
	return m.set.Add(requested)
}

// Unmatched adds a new fsm in unmatched state
func (m *Model) Unmatched() fsm.FSM {
	return m.set.Add(unmatched)
}

// Spec returns the model description
func (m *Model) Spec() *fsm.Spec {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.spec
}

// Start starts the model
func (m *Model) Start() {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.set == nil {
		m.clock.Start()
		m.set = fsm.NewSet(m.spec, m.clock, fsm.DefaultOptions("resource"))
	}
}

// Stop stops the model
func (m *Model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.set != nil {
		m.set.Stop()
		m.clock.Stop()

		close(m.instanceDestroyChan)
		close(m.instanceProvisionChan)
		close(m.instancePendingChan)
		close(m.instanceReadyChan)
		close(m.cleanupChan)
		m.set = nil
	}
}

const (

	// States
	requested fsm.Index = iota
	provisioning
	waiting
	ready
	cannotProvision
	unmatched
	terminating
	terminated

	// Signals
	resourceFound fsm.Signal = iota
	resourceLost
	provision
	provisionError
	dependencyMissing
	dependencyReady
	terminate
	cleanup
)

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties resource.Properties, options resource.Options) (*Model, error) {

	if options.WaitBeforeProvision == 0 {
		panic("bad")
	}

	log.Info("Build model", "properties", properties, "options", options)
	model := &Model{
		Properties:            properties,
		instanceDestroyChan:   make(chan fsm.FSM, options.ChannelBufferSize),
		instanceProvisionChan: make(chan fsm.FSM, options.ChannelBufferSize),
		instancePendingChan:   make(chan fsm.FSM, options.ChannelBufferSize),
		instanceReadyChan:     make(chan fsm.FSM, options.ChannelBufferSize),
		cleanupChan:           make(chan fsm.FSM, options.ChannelBufferSize),
		tickSize:              1 * time.Second,
	}

	// find the max observation interval and set the model tick to be that
	for _, accessor := range properties {
		if model.tickSize < accessor.ObserveInterval.Duration() {
			model.tickSize = accessor.ObserveInterval.Duration()
		}
	}

	log.Info("model", "tickSize", model.tickSize)

	model.clock = fsm.Wall(time.Tick(model.tickSize))

	spec, err := fsm.Define(
		fsm.State{
			Index: requested,
			TTL:   fsm.Expiry{options.WaitBeforeProvision, provision},
			Transitions: map[fsm.Signal]fsm.Index{
				resourceFound: ready,
				resourceLost:  provisioning,
				provision:     provisioning,
			},
			Actions: map[fsm.Signal]fsm.Action{
				provision: func(n fsm.FSM) error {
					model.instanceProvisionChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: provisioning,
			Transitions: map[fsm.Signal]fsm.Index{
				dependencyMissing: waiting,
				resourceFound:     ready,
			},
			Actions: map[fsm.Signal]fsm.Action{
				dependencyMissing: func(n fsm.FSM) error {
					model.instancePendingChan <- n
					return nil
				},
				resourceFound: func(n fsm.FSM) error {
					model.instanceReadyChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: waiting,
			Transitions: map[fsm.Signal]fsm.Index{
				dependencyMissing: waiting,
				dependencyReady:   provisioning,
				provisionError:    cannotProvision,
			},
			Actions: map[fsm.Signal]fsm.Action{
				dependencyReady: func(n fsm.FSM) error {
					model.instanceProvisionChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: ready,
			Transitions: map[fsm.Signal]fsm.Index{
				resourceLost:  provisioning,
				resourceFound: ready, // just loops back to self in the ready state
			},
			Actions: map[fsm.Signal]fsm.Action{
				resourceLost: func(n fsm.FSM) error {
					model.instanceProvisionChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: cannotProvision,
		},
		fsm.State{
			Index: unmatched,
			TTL:   fsm.Expiry{options.WaitBeforeDestroy, terminate},
			Transitions: map[fsm.Signal]fsm.Index{
				terminate: terminating,
			},
			Actions: map[fsm.Signal]fsm.Action{
				terminate: func(n fsm.FSM) error {
					model.instanceDestroyChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: terminating,
			Transitions: map[fsm.Signal]fsm.Index{
				resourceLost: terminated,
			},
		},
		fsm.State{
			Index: terminated,
			TTL:   fsm.Expiry{options.WaitBeforeDestroy, cleanup},
			Transitions: map[fsm.Signal]fsm.Index{
				cleanup: terminated, // TODO - this is really unnecessary
			},
			Actions: map[fsm.Signal]fsm.Action{
				cleanup: func(n fsm.FSM) error {
					model.cleanupChan <- n
					return nil
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	spec.SetStateNames(map[fsm.Index]string{
		requested:       "REQUESTED",
		ready:           "READY",
		provisioning:    "PROVISIONING",
		waiting:         "WAITING",
		cannotProvision: "CANNOT_PROVISION",
		unmatched:       "UNMATCHED",
		terminating:     "TERMINATING",
		terminated:      "TERMINATED",
	}).SetSignalNames(map[fsm.Signal]string{
		resourceFound:     "resource_found",
		resourceLost:      "resource_lost",
		provision:         "provision",
		terminate:         "terminate",
		cleanup:           "cleanup",
		provisionError:    "provision_error",
		dependencyMissing: "dependency_missing",
		dependencyReady:   "dependency_ready",
	})
	model.spec = spec
	return model, nil
}
