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

type options struct {
	TickUnit                  types.Duration
	NoData                    fsm.Tick
	DockerNodeJoin            fsm.Tick
	WaitDescribeInstances     fsm.Tick
	WaitBeforeInstanceDestroy fsm.Tick
	WaitBeforeReprovision     fsm.Tick
	WaitBeforeCleanup         fsm.Tick
}

var defaultOptions = options{
	TickUnit:                  types.FromDuration(1 * time.Second),
	NoData:                    fsm.Tick(10),
	DockerNodeJoin:            fsm.Tick(5),
	WaitDescribeInstances:     fsm.Tick(5),
	WaitBeforeInstanceDestroy: fsm.Tick(3),
	WaitBeforeReprovision:     fsm.Tick(10), // wait before we reprovision a new instance to fix a Down node
	WaitBeforeCleanup:         fsm.Tick(10),
}

type model struct {
	spec       *fsm.Spec
	set        *fsm.Set
	clock      *fsm.Clock
	properties gc_types.Properties
	options    options

	dockerNodeRmChan    chan fsm.Instance
	instanceDestroyChan chan fsm.Instance

	lock sync.RWMutex
}

func (m *model) GCNode() <-chan fsm.Instance {
	return m.dockerNodeRmChan
}

func (m *model) GCInstance() <-chan fsm.Instance {
	return m.instanceDestroyChan
}

func (m *model) New() fsm.Instance {
	return m.set.Add(start)
}

func (m *model) FoundNode(fsm fsm.Instance, desc instance.Description) error {
	// look at node's status - down, ready, etc.
	node, err := NodeFromDescription(desc)
	if err != nil {
		return err
	}

	if node.Status.State != swarm.NodeStateReady {
		fsm.Signal(dockerNodeDown)
		return nil
	}

	if node.Spec.Role == swarm.NodeRoleManager {
		if node.ManagerStatus != nil {
			switch node.ManagerStatus.Reachability {
			case swarm.ReachabilityReachable:
				fsm.Signal(dockerNodeReady)
			default:
				fsm.Signal(dockerNodeDown)
			}
		}
	}

	return fmt.Errorf("unknown state in node %v, no signals triggered", node)
}

func (m *model) LostNode(fsm fsm.Instance) {
	fsm.Signal(dockerNodeGone)
}

func (m *model) FoundInstance(fsm fsm.Instance, desc instance.Description) error {
	fsm.Signal(instanceOK)
	return nil
}

func (m *model) LostInstance(fsm fsm.Instance) {
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

	// Take the longer of the 2 intervals so that the fsm operates on a slower clock.
	d := m.options.TickUnit.Duration()
	if d < m.properties.ObserveInterval.Duration() {
		d = m.properties.ObserveInterval.Duration()
	}
	m.clock = fsm.Wall(time.Tick(d))
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

func (m *model) instanceDestroy(i fsm.Instance) error {
	if m.instanceDestroyChan == nil {
		return fmt.Errorf("not initialized")
	}

	m.instanceDestroyChan <- i
	return nil
}

func (m *model) dockerNodeRm(i fsm.Instance) error {
	if m.dockerNodeRmChan == nil {
		return fmt.Errorf("not initialized")
	}

	m.dockerNodeRmChan <- i
	return nil
}

// BuildModel constructs a workflow model given the configuration blob provided by user in the Properties
func BuildModel(properties gc_types.Properties) (gc.Model, error) {

	modelProperties := properties.ModelProperties

	options := defaultOptions
	if modelProperties != nil {
		if err := modelProperties.Decode(&options); err != nil {
			return nil, err
		}
	}

	model := &model{
		properties:          properties,
		options:             options,
		dockerNodeRmChan:    make(chan fsm.Instance, 10),
		instanceDestroyChan: make(chan fsm.Instance, 10),
	}

	spec, err := fsm.Define(
		fsm.State{
			Index: start,
			TTL:   fsm.Expiry{options.NoData, timeout},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: matchedDockerNode,
				dockerNodeDown:  matchedDockerNode,
				instanceOK:      matchedInstance,
				timeout:         removedInstance, // nothing happened... cleanup
			},
		},
		fsm.State{
			Index: matchedInstance,
			TTL:   fsm.Expiry{options.DockerNodeJoin, dockerNodeGone},
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
			TTL:   fsm.Expiry{options.WaitBeforeInstanceDestroy, reap},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNode, // late joiner
				dockerNodeDown:  swarmNode,
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
			TTL:   fsm.Expiry{options.WaitDescribeInstances, instanceGone},
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
			TTL:   fsm.Expiry{options.WaitBeforeReprovision, dockerNodeGone},
			Transitions: map[fsm.Signal]fsm.Index{
				dockerNodeReady: swarmNodeReady,
				dockerNodeGone:  pendingInstanceDestroy,
				instanceGone:    matchedDockerNode,
			},
		},
		fsm.State{
			Index: removedInstance, // after we removed the instance, we can still have unmatched node
			TTL:   fsm.Expiry{options.WaitBeforeCleanup, timeout},
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

	model.spec = spec
	return model, nil
}
