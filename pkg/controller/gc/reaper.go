package gc

import (
	"context"
	"time"

	gc "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/fsm"
	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

type reaper struct {
	*internal.Collection

	properties gc.Properties
	options    gc.Options

	model Model

	nodeObserver     *internal.InstanceObserver
	instanceObserver *internal.InstanceObserver

	scope     scope.Scope
	nodes     instance.Plugin
	instances instance.Plugin
}

func newReaper(scope scope.Scope, options gc.Options) (internal.Managed, error) {
	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope)
	if err != nil {
		return nil, err
	}

	r := &reaper{
		Collection: base,
		scope:      scope,
		options:    options,
	}

	base.StartFunc = r.run
	base.StopFunc = r.stop
	base.UpdateSpecFunc = r.updateSpec

	return r, nil
}

// Metadata returns an optional metadata.Plugin implementation
func (r *reaper) Metadata() metadata.Plugin {
	return nil
}

// Events returns an optional event.Plugin implementation
func (r *reaper) Events() event.Plugin {
	return nil
}

func (r *reaper) run(ctx context.Context) {

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
}

func (r *reaper) stop() error {
	r.instanceObserver.Stop()
	r.nodeObserver.Stop()
	return nil
}

var (
	defaultInstanceObserver = &internal.InstanceObserver{
		ObserveInterval: types.Duration(1 * time.Second),
	}
)

func (r *reaper) updateSpec(spec types.Spec, prev *types.Spec) error {
	// parse input, then select the model to use
	properties := gc.Properties{}

	err := spec.Properties.Decode(&properties)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err = properties.Validate(ctx); err != nil {
		return err
	}

	model, err := model(properties)
	if err != nil {
		return err
	}

	instanceObserver := properties.InstanceObserver
	if err := instanceObserver.Validate(defaultInstanceObserver); err != nil {
		return err
	}
	if err := instanceObserver.Init(r.scope, r.options.PluginRetryInterval.Duration()); err != nil {
		return err
	}

	nodeObserver := properties.NodeObserver
	if err := nodeObserver.Validate(defaultInstanceObserver); err != nil {
		return err
	}
	if err := nodeObserver.Init(r.scope, r.options.PluginRetryInterval.Duration()); err != nil {
		return err
	}

	r.instanceObserver = &instanceObserver
	r.nodeObserver = &nodeObserver

	r.instances = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(r.instanceObserver.Name.String())
		},
		r.options.PluginRetryInterval.Duration())
	r.nodes = instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return r.scope.Instance(r.nodeObserver.Name.String())
		},
		r.options.PluginRetryInterval.Duration())

	r.properties = properties
	r.model = model
	return nil
}

func (r *reaper) getNodeDescription(i fsm.FSM) (desc *instance.Description) {
	r.Collection.Visit(func(item internal.Item) bool {
		if item.State.ID() == i.ID() {
			copy := (item.Data["node"]).(instance.Description)
			desc = &copy
			return false
		}
		return true
	})
	return
}

func (r *reaper) getInstanceDescription(i fsm.FSM) (desc *instance.Description) {
	r.Collection.Visit(func(item internal.Item) bool {
		if item.State.ID() == i.ID() {
			copy := (item.Data["instance"]).(instance.Description)
			desc = &copy
			return false
		}
		return true
	})
	return
}

func (r *reaper) gc(ctx context.Context) {

	nodeInput := r.model.GCNode()
	instanceInput := r.model.GCInstance()

	for {
		select {

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
	for {
		select {

		case all, ok := <-r.nodeObserver.Lost():
			if !ok {
				return
			}

			for _, lost := range all {
				key, err := r.nodeObserver.KeyOf(lost)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item := r.Collection.Get(key)
				if item != nil {
					r.model.LostNode(item.State)

					r.Collection.Delete(key)
					log.Debug("lostNode", "node", lost, "key", key, "V", debugV)
				}
			}

		case all, ok := <-r.instanceObserver.Lost():
			if !ok {
				return
			}

			for _, lost := range all {
				key, err := r.instanceObserver.KeyOf(lost)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item := r.Collection.Get(key)
				if item != nil {
					r.model.LostInstance(item.State)

					r.Collection.Delete(key)
					log.Debug("lostInstance", "instance", lost, "key", key, "V", debugV)
				}
			}

		case all, ok := <-r.nodeObserver.Observations():
			if !ok {
				return
			}

			for _, found := range all {

				key, err := r.nodeObserver.KeyOf(found)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item := r.Collection.Get(key)
				if item == nil {
					item = r.Collection.Put(key, r.model.New(), r.model.Spec(), nil)
				}

				item.Data["node"] = found // update the node

				r.model.FoundNode(item.State, found) // signal the fsm

				log.Debug("foundNode", "node", found, "V", debugV)
			}

		case all, ok := <-r.instanceObserver.Observations():
			if !ok {
				return
			}

			for _, found := range all {
				key, err := r.instanceObserver.KeyOf(found)
				if err != nil {
					continue // bad data but shouldn't halt everything else
				}

				item := r.Collection.Get(key)
				if item == nil {
					item = r.Collection.Put(key, r.model.New(), r.model.Spec(), nil)
				}

				item.Data["instance"] = found // update the instance

				r.model.FoundInstance(item.State, found) // signal the fsm

				log.Debug("foundInstance", "instance", found, "V", debugV)
			}
		}
	}
}
