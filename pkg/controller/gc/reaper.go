package gc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/controller"
	gc "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/fsm"
	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

type stateMachine struct {
	fsm.FSM
}

func (f stateMachine) MarshalJSON() ([]byte, error) {
	any, err := types.AnyValue(map[string]interface{}{
		"state": f.State(),
	})
	if err != nil {
		return nil, err
	}
	return any.Bytes(), nil
}

type item struct {
	link     string
	instance instance.Description
	node     instance.Description
	fsm      stateMachine
}

type reaper struct {
	spec       types.Spec
	properties gc.Properties
	options    gc.Options
	items      map[string]*item
	stop       chan struct{}

	leader func() stack.Leadership
	scope  scope.Scope
	model  Model

	nodes     []instance.Description // last observation
	instances []instance.Description // last observation

	nodeKeyExtractor     func(instance.Description) (string, error)
	instanceKeyExtractor func(instance.Description) (string, error)

	running bool
	freed   bool
	poller  *controller.Poller
	ticker  <-chan time.Time

	nodeSource     instance.Plugin
	instanceSource instance.Plugin

	lock sync.RWMutex
}

func newReaper(scope scope.Scope, leader func() stack.Leadership, options gc.Options) (internal.Managed, error) {
	r := &reaper{
		leader:  leader,
		scope:   scope,
		options: options,
		items:   map[string]*item{},
		stop:    make(chan struct{}),
	}
	return r, nil
}

// object returns the state
func (r *reaper) object() (*types.Object, error) {
	snapshot, err := r.snapshot()
	if err != nil {
		return nil, err
	}

	object := types.Object{
		Spec:  r.spec,
		State: snapshot,
	}
	return &object, nil
}

// Start starts the reaper
func (r *reaper) Start() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.poller != nil {
		ctx := context.Background()

		r.model.Start()
		log.Info("model started")

		go r.poller.Run(ctx)
		go r.gc(ctx)

		r.running = true
	}
}

// Running returns true if reaper is running
func (r *reaper) Running() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.running
}

func (r *reaper) Stop() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.poller != nil {
		r.poller.Stop()
	}
	return nil
}

func (r *reaper) Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error) {
	o, err := r.object()
	return o, nil, err
}

func (r *reaper) Enforce(spec types.Spec) (*types.Object, error) {
	log.Debug("Enforce", "spec", spec, "V", debugV)

	if err := r.updateSpec(spec); err != nil {
		return nil, err
	}
	r.Start()
	return r.object()
}

func (r *reaper) Inspect() (*types.Object, error) {
	v, err := r.object()
	log.Info("Inspect", "object", *v, "err", err)
	return v, err
}

func (r *reaper) Pause() (*types.Object, error) {
	if r.Running() {
		r.Stop()
	}
	return r.Inspect()
}

func (r *reaper) Free() (*types.Object, error) {
	return r.Pause()
}

func (r *reaper) Terminate() (*types.Object, error) {
	return nil, fmt.Errorf("not supported")
}

func (r *reaper) updateSpec(spec types.Spec) error {
	// parse input, then select the model to use
	properties := gc.Properties{}

	err := spec.Properties.Decode(&properties)
	if err != nil {
		return err
	}

	r.properties = properties

	ctx := context.Background()
	if err := properties.Validate(ctx); err != nil {
		return err
	}

	r.nodeKeyExtractor = internal.KeyExtractor(properties.NodeKeySelector)
	r.instanceKeyExtractor = internal.KeyExtractor(properties.InstanceKeySelector)

	model, err := model(properties)
	if err != nil {
		return err
	}

	r.nodeSource = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(properties.NodeSource.Plugin.String())
		},
		r.options.PluginRetryInterval.Duration())
	r.instanceSource = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(properties.InstanceSource.Plugin.String())
		},
		r.options.PluginRetryInterval.Duration())

	r.spec = spec
	// set identity
	r.spec.Metadata.Identity = &types.Identity{
		ID: r.spec.Metadata.Name,
	}

	r.ticker = time.Tick(properties.ObserveInterval.Duration())
	r.poller = controller.Poll(
		// This determines if the action should be taken when time is up
		func() bool {
			log.Debug("checking before poll", "V", debugV2)
			isLeader := false
			if r.leader != nil {
				v, err := r.leader().IsLeader()
				if err == nil {
					isLeader = v
				}
			}
			log.Debug("polling", "isLeader", isLeader, "V", debugV2, "freed", r.freed)
			return isLeader && !r.freed
		},
		// This does the work
		func() (err error) {
			ctx := context.Background()
			return r.pollAndSignal(ctx)
		},
		r.ticker)

	log.Info("poller", "poll", r.properties.ObserveInterval.Duration())

	r.model = model

	r.freed = false
	return nil
}

func (r *reaper) snapshot() (*types.Any, error) {
	view := []item{}

	for _, item := range r.items {
		obj := *item
		view = append(view, obj)
	}

	return types.AnyValue(view)
}

func (r *reaper) observe(ctx context.Context) (nodes []instance.Description,
	instances []instance.Description, err error) {

	log.Debug("observe start", "V", debugV)

	nodes, err = r.nodeSource.DescribeInstances(r.properties.NodeSource.Labels, true)
	if err != nil {
		return
	}

	instances, err = r.instanceSource.DescribeInstances(r.properties.InstanceSource.Labels, true)

	log.Debug("observe", "V", debugV, "nodes", nodes, "instances", instances)
	return
}

func (r *reaper) getNodeDescription(i fsm.FSM) *instance.Description {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, item := range r.items {
		if item.fsm.ID() == i.ID() {
			desc := item.node
			return &desc
		}
	}
	return nil
}

func (r *reaper) getInstanceDescription(i fsm.FSM) *instance.Description {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, item := range r.items {
		if item.fsm.ID() == i.ID() {
			desc := item.instance
			return &desc
		}
	}
	return nil
}

func (r *reaper) gc(ctx context.Context) {

	nodeInput := r.model.GCNode()
	instanceInput := r.model.GCInstance()

	for {
		select {

		case <-r.stop:
			log.Info("Stop")
			return

		case m, ok := <-nodeInput:
			if !ok {
				log.Info("NodeChan shutting down")
				return
			}

			t := r.getNodeDescription(m)
			if t != nil {
				err := r.nodeSource.Destroy(t.ID, instance.Termination)

				log.Debug("nodeDestroy", "id", t.ID, "node", t, "V", debugV)

				if err != nil {
					log.Error("error destroying node", "err", err, "id", t.ID)
				}
			}

		case m, ok := <-instanceInput:
			if !ok {
				log.Info("InstanceChan shutting down")
				return
			}

			t := r.getInstanceDescription(m)
			if t != nil {
				err := r.instanceSource.Destroy(t.ID, instance.Termination)

				log.Debug("instanceDestroy", "id", t.ID, "instance", t, "V", debugV)

				if err != nil {
					log.Error("error destroying instance", "err", err, "id", t.ID)
				}
			}
		}
	}
}

func (r *reaper) pollAndSignal(ctx context.Context) error {
	log.Debug("polling starts", "V", debugV)

	nodes, instances, err := r.observe(ctx)
	if err != nil {
		return err
	}

	// update the observations for the next poll
	defer func() {
		r.instances = instances
		r.nodes = nodes
	}()

	for _, instance := range instances {
		key, err := r.instanceKeyExtractor(instance)
		if err != nil {
			continue // bad data but shouldn't halt everything else
		}

		found, has := r.items[key]
		if !has {
			r.items[key] = &item{
				instance: instance,
				fsm:      stateMachine{r.model.New()},
			}
		} else {
			r.model.FoundInstance(found.fsm, instance)
			found.instance = instance

			log.Debug("foundInstance", "instance", instance, "V", debugV)
		}
	}

	for _, node := range nodes {
		key, err := r.nodeKeyExtractor(node)
		if err != nil {
			continue // bad data but shouldn't halt everything else
		}

		found, has := r.items[key]
		if !has {
			r.items[key] = &item{
				node: node,
				fsm:  stateMachine{r.model.New()},
			}
		} else {
			r.model.FoundNode(found.fsm, node)
			found.node = node

			log.Debug("foundNode", "node", node, "V", debugV)
		}
	}

	// compute the lost nodes / instances
	if len(r.nodes) > 0 {

		diff := instance.Difference(
			instance.Descriptions(r.nodes), r.nodeKeyExtractor,
			instance.Descriptions(nodes), r.nodeKeyExtractor)

		for _, lost := range diff {

			key, err := r.nodeKeyExtractor(lost)
			if err != nil {
				continue // bad data but shouldn't halt everything else
			}

			item, has := r.items[key]
			if has {
				r.model.LostNode(item.fsm)
				delete(r.items, key)

				log.Debug("lostNode", "node", lost, "key", key, "V", debugV)
			}
		}
	}

	if len(r.instances) > 0 {

		diff := instance.Difference(
			instance.Descriptions(r.instances), r.instanceKeyExtractor,
			instance.Descriptions(instances), r.instanceKeyExtractor)

		for _, lost := range diff {

			key, err := r.instanceKeyExtractor(lost)
			if err != nil {
				continue // bad data but shouldn't halt everything else
			}

			item, has := r.items[key]
			if has {
				r.model.LostInstance(item.fsm)
				delete(r.items, key)

				log.Debug("lostInstance", "instance", lost, "key", key, "V", debugV)
			}
		}
	}

	return nil
}
