package swarm

import (
	"sync"

	"github.com/docker/infrakit/pkg/controller/gc"
	. "github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
)

var (
	log    = logutil.New("module", "controller/gc/swarm")
	debugV = logutil.V(300)
)

func init() {
	gc.Register("swarm", BuildModel)
}

const (
	dockerNodeReady Signal = iota
	dockerNodeDown
	dockerNodeGone
	instanceOK
	instanceGone
	timeout
	reap
)

const (
	start                  Index = iota
	matchedInstance              // has vm information, waiting to match to docker_node
	matchedDockerNode            // has docker_node information, waiting to match to vm
	swarmNode                    // has matching docker_node and vm information
	swarmNodeReady               // ready as swarm node
	swarmNodeDown                // unavailable as swarm node
	pendingInstanceDestroy       // vm needs to be removedInstance (instance destroy)
	removedInstance              // instance is deleted
	done                         // terminal
)

type model struct {
	spec *Spec
	set  *Set
	lock sync.RWMutex
}

func (m *model) New() Instance {
	return m.set.Add(start)
}

func (m *model) FoundNode(fsm Instance, desc instance.Description) {
	// look at node's status - down, ready, etc.
}

func (m *model) LostNode(fsm Instance) {
	fsm.Signal(dockerNodeGone)
}

func (m *model) FoundInstance(fsm Instance, desc instance.Description) {
	fsm.Signal(instanceOK)
}

func (m *model) LostInstance(fsm Instance) {
	fsm.Signal(instanceGone)
}

func (m *model) Spec() *Spec {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.spec
}

func (m *model) Start() {
	m.lock.Lock()
	defer m.lock.Unlock()

}

func (m *model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
}

func (m *model) instanceDestroy(i Instance) error {
	return nil
}

func (m *model) dockerNodeRm(i Instance) error {
	return nil
}

func BuildModel() (gc.Model, error) {

	noData := Tick(10)
	dockerNodeJoin := Tick(5)
	waitDescribeInstances := Tick(5)
	waitBeforeInstanceDestroy := Tick(3)
	waitBeforeReprovision := Tick(10) // wait before we reprovision a new instance to fix a Down node
	waitBeforeCleanup := Tick(10)

	model := &model{}

	spec, err := Define(
		State{
			Index: start,
			TTL:   Expiry{noData, timeout},
			Transitions: map[Signal]Index{
				dockerNodeReady: matchedDockerNode,
				dockerNodeDown:  matchedDockerNode,
				instanceOK:      matchedInstance,
				timeout:         removedInstance, // nothing happened... cleanup
			},
		},
		State{
			Index: matchedInstance,
			TTL:   Expiry{dockerNodeJoin, dockerNodeGone},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNode,
				dockerNodeDown:  swarmNode,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    removedInstance,
			},
			Actions: map[Signal]Action{
				instanceGone: model.instanceDestroy,
			},
		},
		State{
			Index: pendingInstanceDestroy,
			TTL:   Expiry{waitBeforeInstanceDestroy, reap},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNode, // late joiner
				dockerNodeDown:  swarmNode,
				instanceGone:    removedInstance,
				reap:            removedInstance,
			},
			Actions: map[Signal]Action{
				instanceGone: model.instanceDestroy,
				reap:         model.instanceDestroy,
			},
		},
		State{
			Index: matchedDockerNode,
			TTL:   Expiry{waitDescribeInstances, instanceGone},
			Transitions: map[Signal]Index{
				instanceOK:     swarmNode,
				instanceGone:   removedInstance,
				dockerNodeGone: removedInstance, // could be docker rm'd out of band
			},
			Actions: map[Signal]Action{
				instanceGone: model.dockerNodeRm,
			},
		},
		State{
			Index: swarmNode,
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeDown:  swarmNodeDown,
				dockerNodeGone:  matchedInstance,
				instanceGone:    matchedDockerNode,
			},
		},
		State{
			Index: swarmNodeReady,
			Transitions: map[Signal]Index{
				dockerNodeDown: swarmNodeDown,
				dockerNodeGone: matchedInstance,
				instanceGone:   matchedDockerNode,
			},
		},
		State{
			Index: swarmNodeDown,
			TTL:   Expiry{waitBeforeReprovision, dockerNodeGone},
			Transitions: map[Signal]Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    matchedDockerNode,
			},
		},
		State{
			Index: removedInstance, // after we removed the instance, we can still have unmatched node
			TTL:   Expiry{waitBeforeCleanup, timeout},
			Transitions: map[Signal]Index{
				dockerNodeDown: done,
				timeout:        done,
			},
			Actions: map[Signal]Action{
				dockerNodeDown: model.dockerNodeRm,
			},
		},
		State{
			Index: done, // deleted state is terminal. this will be garbage collected
		},
	)

	if err != nil {
		return nil, err
	}

	model.spec = spec
	return model, nil
}
