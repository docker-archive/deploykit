package swarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/controller/gc"
	gc_types "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log    = logutil.New("module", "controller/gc/swarm")
	debugV = logutil.V(300)
)

func init() {
	gc.Register("swarm", BuildModel)
}

const (
	dockerNodeReady fsm.Signal = iota
	dockerNodeDown
	dockerNodeGone
	instanceOK
	instanceGone
	timeout
	reap
)

// NodeFromDescription returns a docker node that is assumed to be attached as a Properties
func NodeFromDescription(desc instance.Description) (swarm.Node, error) {
	node := swarm.Node{}
	if desc.Properties == nil {
		return node, fmt.Errorf("no docker node information %v", desc)
	}
	return node, desc.Properties.Decode(&node)
}

const (
	start                  fsm.Index = iota
	matchedInstance                  // has vm information, waiting to match to docker_node
	matchedDockerNode                // has docker_node information, waiting to match to vm
	swarmNode                        // has matching docker_node and vm information
	swarmNodeReady                   // ready as swarm node
	swarmNodeDown                    // unavailable as swarm node
	pendingInstanceDestroy           // vm needs to be removedInstance (instance destroy)
	removedInstance                  // instance is deleted
	done                             // terminal
)

type modelProperties struct {
	TickUnit                  types.Duration
	NoData                    fsm.Tick
	DockerNodeJoin            fsm.Tick
	WaitDescribeInstances     fsm.Tick
	WaitBeforeInstanceDestroy fsm.Tick
	WaitBeforeReprovision     fsm.Tick
	WaitBeforeCleanup         fsm.Tick
	RmNodeBufferSize          int
	RmInstanceBufferSize      int
}

var defaultModelProperties = modelProperties{
	TickUnit:                  types.FromDuration(1 * time.Second),
	NoData:                    fsm.Tick(10),
	DockerNodeJoin:            fsm.Tick(5),
	WaitDescribeInstances:     fsm.Tick(5),
	WaitBeforeInstanceDestroy: fsm.Tick(3),
	WaitBeforeReprovision:     fsm.Tick(10), // wait before we reprovision a new instance to fix a Down node
	WaitBeforeCleanup:         fsm.Tick(10),
	RmNodeBufferSize:          10,
	RmInstanceBufferSize:      10,
}

type model struct {
	spec     *fsm.Spec
	set      *fsm.Set
	clock    *fsm.Clock
	tickSize time.Duration

	gc_types.Properties
	modelProperties

	dockerNodeRmChan    chan fsm.FSM
	instanceDestroyChan chan fsm.FSM

	lock sync.RWMutex
}

func (m *model) GCNode() <-chan fsm.FSM {
	return m.dockerNodeRmChan
}

func (m *model) GCInstance() <-chan fsm.FSM {
	return m.instanceDestroyChan
}

func (m *model) New() fsm.FSM {
	return m.set.Add(start)
}

func (m *model) FoundNode(fsm fsm.FSM, desc instance.Description) error {
	// look at node's status - down, ready, etc.
	node, err := NodeFromDescription(desc)
	if err != nil {
		return err
	}

	if node.Status.State != swarm.NodeStateReady {
		fsm.Signal(dockerNodeDown)
		log.Error("swarm node down", "node", node)
		return nil
	}

	if node.Spec.Role == swarm.NodeRoleManager {
		if node.ManagerStatus != nil {
			switch node.ManagerStatus.Reachability {
			case swarm.ReachabilityReachable:
				fsm.Signal(dockerNodeReady)
				return nil
			default:
				fsm.Signal(dockerNodeDown)
				log.Error("swarm manager node down", "node", node)
				return nil
			}
		}
	}

	if node.Status.State == swarm.NodeStateReady {
		fsm.Signal(dockerNodeReady)
	}

	return fmt.Errorf("unknown state in node %v, no signals triggered", node)
}

func (m *model) LostNode(fsm fsm.FSM) {
	fsm.Signal(dockerNodeGone)
}

func (m *model) FoundInstance(fsm fsm.FSM, desc instance.Description) error {
	fsm.Signal(instanceOK)
	return nil
}

func (m *model) LostInstance(fsm fsm.FSM) {
	fsm.Signal(instanceGone)
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
	m.set = fsm.NewSet(m.spec, m.clock, fsm.DefaultOptions("swarm"))
}

func (m *model) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.set.Stop()
	m.clock.Stop()

	close(m.dockerNodeRmChan)
	close(m.instanceDestroyChan)
}

func (m *model) instanceDestroy(i fsm.FSM) error {
	if m.instanceDestroyChan == nil {
		return fmt.Errorf("not initialized")
	}

	m.instanceDestroyChan <- i
	return nil
}

func (m *model) dockerNodeRm(i fsm.FSM) error {
	if m.dockerNodeRmChan == nil {
		return fmt.Errorf("not initialized")
	}

	m.dockerNodeRmChan <- i
	return nil
}

func longer(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties gc_types.Properties) (gc.Model, error) {

	modelProperties := defaultModelProperties
	if properties.ModelProperties != nil {
		if err := properties.ModelProperties.Decode(&modelProperties); err != nil {
			return nil, err
		}
	}

	model := &model{
		Properties:          properties,
		modelProperties:     modelProperties,
		dockerNodeRmChan:    make(chan fsm.FSM, modelProperties.RmNodeBufferSize),
		instanceDestroyChan: make(chan fsm.FSM, modelProperties.RmInstanceBufferSize),
	}

	d := longer(model.modelProperties.TickUnit.Duration(), longer(
		properties.NodeObserver.ObserveInterval.Duration(),
		properties.InstanceObserver.ObserveInterval.Duration(),
	))

	model.tickSize = d
	model.clock = fsm.Wall(time.Tick(d))

	spec, err := fsm.Define(
		fsm.State{
			Index: start,
			TTL:   fsm.Expiry{modelProperties.NoData, timeout},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: matchedDockerNode,
				dockerNodeDown:  matchedDockerNode,
				instanceOK:      matchedInstance,
				timeout:         removedInstance, // nothing happened... cleanup
			},
		},
		fsm.State{
			Index: matchedInstance,
			TTL:   fsm.Expiry{modelProperties.DockerNodeJoin, dockerNodeGone},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNode,
				dockerNodeDown:  swarmNode,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    removedInstance,
			},
			Actions: map[fsm.Signal]fsm.Action{
				instanceGone: model.instanceDestroy,
			},
		},
		fsm.State{
			Index: pendingInstanceDestroy,
			TTL:   fsm.Expiry{modelProperties.WaitBeforeInstanceDestroy, reap},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNode, // late joiner
				instanceGone:    removedInstance,
				reap:            removedInstance,
			},
			Actions: map[fsm.Signal]fsm.Action{
				instanceGone: model.instanceDestroy,
				reap:         model.instanceDestroy,
			},
		},
		fsm.State{
			Index: matchedDockerNode,
			TTL:   fsm.Expiry{modelProperties.WaitDescribeInstances, instanceGone},
			Transitions: map[fsm.Signal]fsm.Index{
				instanceOK:     swarmNode,
				instanceGone:   removedInstance,
				dockerNodeGone: removedInstance, // could be docker rm'd out of band
			},
			Actions: map[fsm.Signal]fsm.Action{
				instanceGone: model.dockerNodeRm,
			},
		},
		fsm.State{
			Index: swarmNode,
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeDown:  swarmNodeDown,
				dockerNodeGone:  matchedInstance,
				instanceGone:    matchedDockerNode,
			},
		},
		fsm.State{
			Index: swarmNodeReady,
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeDown: swarmNodeDown,
				dockerNodeGone: matchedInstance,
				instanceGone:   matchedDockerNode,
			},
		},
		fsm.State{
			Index: swarmNodeDown,
			TTL:   fsm.Expiry{modelProperties.WaitBeforeReprovision, dockerNodeGone},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    matchedDockerNode,
			},
		},
		fsm.State{
			Index: removedInstance, // after we removed the instance, we can still have unmatched node
			TTL:   fsm.Expiry{modelProperties.WaitBeforeCleanup, timeout},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeDown: done,
				timeout:        done,
			},
			Actions: map[fsm.Signal]fsm.Action{
				dockerNodeDown: model.dockerNodeRm,
			},
		},
		fsm.State{
			Index: done, // deleted state is terminal. this will be garbage collected
		},
	)

	if err != nil {
		return nil, err
	}

	spec.SetStateNames(map[fsm.Index]string{
		start:                  "START",
		matchedInstance:        "FOUND_INSTANCE",
		matchedDockerNode:      "FOUND_DOCKER_NODE",
		swarmNode:              "SWARM_NODE",
		swarmNodeReady:         "SWARM_NODE_READY",
		swarmNodeDown:          "SWARM_NODE_DOWN",
		pendingInstanceDestroy: "PEDNING_INSTANCE_DESTROY",
		removedInstance:        "INSTANCE_REMOVED",
		done:                   "DONE",
	}).SetSignalNames(map[fsm.Signal]string{
		dockerNodeReady: "docker_node_ready",
		dockerNodeDown:  "docker_node_down",
		dockerNodeGone:  "docker_node_gone",
		instanceOK:      "instance_ok",
		instanceGone:    "instance_gone",
		timeout:         "timeout",
		reap:            "reap",
	})
	model.spec = spec
	return model, nil
}
