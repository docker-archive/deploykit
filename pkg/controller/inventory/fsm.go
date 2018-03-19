package inventory

import (
	"sync"
	"time"

	inventory "github.com/docker/infrakit/pkg/controller/inventory/types"
	"github.com/docker/infrakit/pkg/fsm"
)

// Model encapsulates the workflow / state machines for provisioning resources
type Model struct {
	spec     *fsm.Spec
	set      *fsm.Set
	clock    *fsm.Clock
	tickSize time.Duration

	inventory.Properties

	instanceFoundChan chan fsm.FSM
	instanceLostChan  chan fsm.FSM

	lock sync.RWMutex
}

// Found is the channel to get signals of instances that are found
func (m *Model) Found() <-chan fsm.FSM {
	return m.instanceFoundChan
}

// Lost is the channel to get signals of instances that are lost
func (m *Model) Lost() <-chan fsm.FSM {
	return m.instanceLostChan
}

// New adds a new fsm in the found (initial) state
func (m *Model) New() fsm.FSM {
	return m.set.Add(found)
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
		m.set = fsm.NewSet(m.spec, m.clock, fsm.DefaultOptions("inventory"))
	}
}

// Stop stops the model
func (m *Model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.set != nil {
		m.set.Stop()
		m.clock.Stop()

		close(m.instanceFoundChan)
		close(m.instanceLostChan)
		m.set = nil
	}

}

const (

	// States
	found fsm.Index = iota
	lost

	// Signals
	resourceFound fsm.Signal = iota
	resourceLost
)

var (
	stateNames = map[fsm.Index]string{
		found: "FOUND",
		lost:  "LOST",
	}

	signalNames = map[fsm.Signal]string{
		resourceFound: "resource_found",
		resourceLost:  "resource_lost",
	}
)

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties inventory.Properties, options inventory.Options) (*Model, error) {

	log.Info("Build model", "properties", properties)
	model := &Model{
		Properties:        properties,
		instanceFoundChan: make(chan fsm.FSM, options.ChannelBufferSize),
		instanceLostChan:  make(chan fsm.FSM, options.ChannelBufferSize),
		tickSize:          1 * time.Second,
	}

	// find the max observation interval and set the model tick to be that
	for _, accessList := range properties {
		for _, accessor := range accessList {
			if model.tickSize < accessor.ObserveInterval.Duration() {
				model.tickSize = accessor.ObserveInterval.Duration()
			}
		}
	}

	// We must guarantee that the tick size is at least as large as the global
	// setting.  This is so that we don't miss samples and instead advances state
	// too quickly.
	if options.InstanceObserver.ObserveInterval.Duration() > model.tickSize {
		model.tickSize = options.InstanceObserver.ObserveInterval.Duration()
	}

	log.Info("model", "tickSize", model.tickSize)

	model.clock = fsm.Wall(time.Tick(model.tickSize))
	spec, err := fsm.With(stateNames, signalNames).Define(
		fsm.State{
			Index: found,
			Transitions: map[fsm.Signal]fsm.Index{
				resourceFound: found,
				resourceLost:  lost,
			},
			Actions: map[fsm.Signal]fsm.Action{
				resourceLost: func(n fsm.FSM) error {
					model.instanceLostChan <- n
					return nil
				},
			},
		},
		fsm.State{
			Index: lost,
			Transitions: map[fsm.Signal]fsm.Index{
				resourceFound: found,
			},
			Actions: map[fsm.Signal]fsm.Action{
				resourceFound: func(n fsm.FSM) error {
					model.instanceFoundChan <- n
					return nil
				},
			},
		},
	)

	log.Info("model", "tickSize", model.tickSize, "err", err)
	if err != nil {
		panic(err) // Panic because there's a problem with the static / code definition of the model
	}

	model.spec = spec
	return model, nil
}
