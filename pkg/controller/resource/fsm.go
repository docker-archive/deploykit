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
	TickUnit:                    types.FromDuration(1 * time.Second),
	WaitBeforeProvision:         fsm.Tick(10),
	InstanceProvisionBufferSize: 10,
	InstanceDestroyBufferSize:   10,
}

type model struct {
	spec     *fsm.Spec
	set      *fsm.Set
	clock    *fsm.Clock
	tickSize time.Duration

	resource.Properties

	instanceDestroyChan   chan fsm.FSM
	instanceProvisionChan chan fsm.FSM
	instanceFoundChan     chan fsm.FSM
	instanceLostChan      chan fsm.FSM

	lock sync.RWMutex
}

func (m *model) Found() chan<- fsm.FSM {
	return m.instanceFoundChan
}

func (m *model) Lost() chan<- fsm.FSM {
	return m.instanceLostChan
}

func (m *model) Destroy() <-chan fsm.FSM {
	return m.instanceDestroyChan
}

func (m *model) Provision() <-chan fsm.FSM {
	return m.instanceProvisionChan
}

func (m *model) New() fsm.FSM {
	return m.set.Add(requested)
}

func (m *model) Spec() *fsm.Spec {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.spec
}

func (m *model) Start() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.clock.Start()
	m.set = fsm.NewSet(m.spec, m.clock, fsm.DefaultOptions("resource"))
	go func() {
		for {
			select {

			case resource, ok := <-m.instanceFoundChan:
				if !ok {
					return
				}
				resource.Signal(resourceFound)
			case resource, ok := <-m.instanceLostChan:
				if !ok {
					return
				}
				resource.Signal(resourceLost)
			}
		}
	}()
}

func (m *model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.set.Stop()
	m.clock.Stop()

	close(m.instanceDestroyChan)
	close(m.instanceProvisionChan)
	close(m.instanceFoundChan)
	close(m.instanceLostChan)
}

func (m *model) instanceDestroy(i fsm.FSM) error {
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

	// Signals
	resourceFound fsm.Signal = iota
	resourceLost
	provision
	provisionError
	dependencyMissing
	dependencyReady
)

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties resource.Properties) (resource.Model, error) {

	model := &model{
		Properties:            properties,
		instanceDestroyChan:   make(chan fsm.FSM, properties.InstanceDestroyBufferSize),
		instanceProvisionChan: make(chan fsm.FSM, properties.InstanceProvisionBufferSize),
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
	}).SetSignalNames(map[fsm.Signal]string{
		resourceFound:     "resource_found",
		resourceLost:      "resource_lost",
		provision:         "provision",
		provisionError:    "provision_error",
		dependencyMissing: "dependency_missing",
		dependencyReady:   "dependency_ready",
	})
	model.spec = spec
	return model, nil
}
