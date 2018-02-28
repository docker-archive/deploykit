package resource

import (
	"fmt"
	"sync"
	"time"

	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/types"
)

var defaultModelProperties = resource.ModelProperties{
	TickUnit:            types.FromDuration(1 * time.Second),
	WaitBeforeProvision: fsm.Tick(10),
	WaitBeforeDestroy:   fsm.Tick(10),
	ChannelBufferSize:   10,
}

// Model encapsulates the workflow / state machines for provisioning resources
type Model struct {
	spec     *fsm.Spec
	set      *fsm.Set
	clock    *fsm.Clock
	tickSize time.Duration

	resource.Properties

	instanceDestroyChan   chan fsm.FSM
	instanceProvisionChan chan fsm.FSM

	lock sync.RWMutex
}

// Destroy is the channel to get signals to destroy an instance
func (m *Model) Destroy() <-chan fsm.FSM {
	return m.instanceDestroyChan
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

	m.clock.Start()
	m.set = fsm.NewSet(m.spec, m.clock, fsm.DefaultOptions("resource"))
}

func (m *Model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.set.Stop()
	m.clock.Stop()

	close(m.instanceDestroyChan)
	close(m.instanceProvisionChan)
}

func (m *Model) instanceDestroy(i fsm.FSM) error {
	if m.instanceDestroyChan == nil {
		return fmt.Errorf("not initialized")
	}

	m.instanceDestroyChan <- i
	return nil
}

func longest(t ...time.Duration) time.Duration {
	var max time.Duration
	for _, tt := range t {
		if tt > max {
			max = tt
		}
	}
	return max
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
)

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties resource.Properties) (*Model, error) {

	model := &Model{
		Properties:            properties,
		instanceDestroyChan:   make(chan fsm.FSM, properties.ChannelBufferSize),
		instanceProvisionChan: make(chan fsm.FSM, properties.ChannelBufferSize),
		tickSize:              1 * time.Second,
	}

	// find the max observation interval and set the model tick to be that
	for _, accessor := range properties.Resources {
		if model.tickSize < accessor.ObserveInterval.Duration() {
			model.tickSize = accessor.ObserveInterval.Duration()
		}
	}

	model.clock = fsm.Wall(time.Tick(model.tickSize))

	spec, err := fsm.Define(
		fsm.State{
			Index: requested,
			TTL:   fsm.Expiry{properties.WaitBeforeProvision, provision},
			Transitions: map[fsm.Signal]fsm.Index{
				resourceFound: ready,
				resourceLost:  provisioning,
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
				resourceFound: ready,
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
			TTL:   fsm.Expiry{properties.WaitBeforeDestroy, terminate},
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
		provisionError:    "provision_error",
		dependencyMissing: "dependency_missing",
		dependencyReady:   "dependency_ready",
	})
	model.spec = spec
	return model, nil
}
