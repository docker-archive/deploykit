package gc

import (
	"context"
	"fmt"
	"sync"
	"time"

	gc "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/fsm"
	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

type stateMachine struct {
	fsm.FSM
	*fsm.Spec
}

func (f stateMachine) MarshalJSON() ([]byte, error) {
	any, err := types.AnyValue(map[string]interface{}{
		"state": f.StateName(f.State()),
	})
	if err != nil {
		return nil, err
	}
	return any.Bytes(), nil
}

type item struct {
	Link     string
	Instance instance.Description
	Node     instance.Description
	FSM      stateMachine
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

	nodeObserver     *internal.InstanceObserver
	instanceObserver *internal.InstanceObserver

	running bool
	freed   bool
	poller  *internal.Poller
	ticker  <-chan time.Time

	nodes     instance.Plugin
	instances instance.Plugin

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

	r.spec.Metadata.Identity = &types.Identity{
		ID: r.spec.Metadata.Name,
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

	r.start()
}

func (r *reaper) start() {
	ctx := context.Background()

	r.model.Start()
	log.Info("model started")

	go r.instanceObserver.Start()
	log.Info("instance started")

	go r.nodeObserver.Start()
	log.Info("node started")

	go r.gc(ctx)
	log.Info("gc started")

	go r.processObservations(ctx)
	log.Info("processing observations")

	r.running = true
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

	r.instanceObserver.Stop()
	r.nodeObserver.Stop()
	return nil
}

func (r *reaper) Plan(controller.Operation, types.Spec) (*types.Object, *controller.Plan, error) {
	o, err := r.object()
	return o, nil, err
}

func (r *reaper) Enforce(spec types.Spec) (*types.Object, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	log.Debug("Enforce", "spec", spec, "V", debugV)

	if err := r.updateSpec(spec); err != nil {
		return nil, err
	}
	r.start()
	return r.object()
}

func (r *reaper) Inspect() (*types.Object, error) {
	v, err := r.object()
	log.Info("Inspect", "object", *v, "err", err)
	return v, err
}

func (r *reaper) Pause() (*types.Object, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.instanceObserver.Pause(true)
	r.nodeObserver.Pause(true)
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

	ctx := context.Background()
	if err := properties.Validate(ctx); err != nil {
		return err
	}

	model, err := model(properties)
	if err != nil {
		return err
	}

	instanceObserver := properties.InstanceObserver
	if err := instanceObserver.Init(r.scope, r.leader, r.options.PluginRetryInterval.Duration()); err != nil {
		return err
	}

	nodeObserver := properties.NodeObserver
	if err := nodeObserver.Init(r.scope, r.leader, r.options.PluginRetryInterval.Duration()); err != nil {
		return err
	}

	r.instanceObserver = &instanceObserver
	r.nodeObserver = &nodeObserver

	r.instances = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(r.instanceObserver.Plugin.String())
		},
		r.options.PluginRetryInterval.Duration())
	r.nodes = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(r.nodeObserver.Plugin.String())
		},
		r.options.PluginRetryInterval.Duration())

	// set identity
	r.spec.Metadata.Identity = &types.Identity{
		ID: r.spec.Metadata.Name,
	}
	r.freed = false
	r.properties = properties
	r.model = model
	r.spec = spec
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

func (r *reaper) getNodeDescription(i fsm.FSM) *instance.Description {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, item := range r.items {
		if item.FSM.ID() == i.ID() {
			desc := item.Node
			return &desc
		}
	}
	return nil
}

func (r *reaper) getInstanceDescription(i fsm.FSM) *instance.Description {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, item := range r.items {
		if item.FSM.ID() == i.ID() {
			desc := item.Instance
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
				err := r.nodes.Destroy(t.ID, instance.Termination)

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
				err := r.instances.Destroy(t.ID, instance.Termination)

				log.Debug("instanceDestroy", "id", t.ID, "instance", t, "V", debugV)

				if err != nil {
					log.Error("error destroying instance", "err", err, "id", t.ID)
				}
			}
		}
	}
}

func (r *reaper) processObservations(ctx context.Context) {
	nodes, instances := []instance.Description{}, []instance.Description{}

	for {
		select {
		case found, ok := <-r.nodeObserver.Observations():
			if !ok {
				return
			}

			for _, node := range found {
				key, err := r.nodeObserver.KeyOf(node)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				found, has := r.items[key]
				if !has {
					r.items[key] = &item{
						Link: key,
						Node: node,
						FSM:  stateMachine{r.model.New(), r.model.Spec()},
					}
				} else {
					r.model.FoundNode(found.FSM, node)
					found.Node = node

					log.Debug("foundNode", "node", node, "V", debugV)
				}
			}

			// differences for lost nodes
			for _, lost := range instance.Difference(
				instance.Descriptions(nodes), r.nodeObserver.KeyOf,
				instance.Descriptions(found), r.nodeObserver.KeyOf,
			) {

				key, err := r.nodeObserver.KeyOf(lost)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item, has := r.items[key]
				if has {
					r.model.LostNode(item.FSM)
					delete(r.items, key)

					log.Debug("lostNode", "node", lost, "key", key, "V", debugV)
				}
			}

			nodes = found

		case found, ok := <-r.instanceObserver.Observations():
			if !ok {
				return
			}

			for _, instance := range found {
				key, err := r.instanceObserver.KeyOf(instance)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				found, has := r.items[key]
				if !has {
					r.items[key] = &item{
						Link:     key,
						Instance: instance,
						FSM:      stateMachine{r.model.New(), r.model.Spec()},
					}
				} else {
					r.model.FoundInstance(found.FSM, instance)
					found.Instance = instance

					log.Debug("foundInstance", "instance", instance, "V", debugV)
				}
			}

			// differences for lost nodes
			for _, lost := range instance.Difference(
				instance.Descriptions(instances), r.instanceObserver.KeyOf,
				instance.Descriptions(found), r.instanceObserver.KeyOf,
			) {

				key, err := r.instanceObserver.KeyOf(lost)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item, has := r.items[key]
				if has {
					r.model.LostInstance(item.FSM)
					delete(r.items, key)

					log.Debug("lostInstance", "instance", lost, "key", key, "V", debugV)
				}
			}

			instances = found
		}
	}
}
