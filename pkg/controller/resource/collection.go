package resource

import (
	"context"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

type collection struct {
	*internal.Collection

	properties resource.Properties
	options    resource.Options

	instancePlugins map[string]*internal.InstanceObserver
}

func newCollection(scope scope.Scope, leader func() stack.Leadership,
	options resource.Options) (internal.Managed, error) {

	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope, leader)
	if err != nil {
		return nil, err
	}
	c := &collection{
		Collection:      base,
		options:         options,
		instancePlugins: map[string]*internal.InstanceObserver{},
	}

	// set the behaviors
	base.StartFunc = c.start
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec
	return c, nil
}

func (c *collection) start(ctx context.Context) {
	log.Info("starting")

	// channels that aggregate from all the instance accessors
	type event struct {
		name      string
		instances []instance.Description
	}
	allLost := make(chan *event, 100)
	allFound := make(chan *event, 100)

	accessors := map[string]*internal.InstanceAccess(c.properties)

	// Start all the instance accessors and wire up the events.
	for k, a := range accessors {

		name := k
		accessor := a

		log.Debug("Set up events from instance accessor", "name", name, "V", debugV)
		go func() {

			for {
				select {
				case list, ok := <-accessor.Observations():
					if !ok {
						log.Debug("found events done", "name", name, "V", debugV)
						return
					}
					allFound <- &event{name: name, instances: list}

				case list, ok := <-accessor.Lost():
					if !ok {
						log.Debug("lost events done", "name", name, "V", debugV)
						return
					}
					allLost <- &event{name: name, instances: list}
				}
			}
		}()

		// start
		accessor.Start()
	}

	go func() {
		for {

			select {
			case lost, ok := <-allLost:
				if !ok {
					log.Info("Lost aggregator done")
					return
				}

				accessor, has := accessors[lost.name]
				if !has {
					log.Warn("cannot find accessor for lost instance", "name", lost.name)
					break
				}
				for _, n := range lost.instances {
					k, err := accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}

					log.Info("lost", "instance", n, "name", lost.name, "key", k)
				}

			case found, ok := <-allFound:
				if !ok {
					log.Info("Found aggregator done")
					return
				}

				accessor, has := accessors[found.name]
				if !has {
					log.Warn("cannot find accessor for found instance", "name", found.name)
					break
				}
				for _, n := range found.instances {
					k, err := accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}
					log.Info("found", "instance", n, "name", found.name, "key", k)
				}
			}
		}
	}()
}

func (c *collection) stop() error {
	log.Info("stop")

	for k, accessor := range c.properties {
		log.Debug("Stopping", "name", k, "V", debugV)
		accessor.Stop()
	}
	return nil
}

func (c *collection) updateSpec(spec types.Spec) (err error) {

	defer log.Debug("updateSpec", "spec", spec, "err", err)

	// parse input, then select the model to use
	properties := resource.Properties{}
	err = spec.Properties.Decode(&properties)
	if err != nil {
		return
	}

	ctx := context.Background()
	if err = properties.Validate(ctx); err != nil {
		return
	}

	// init the instance accessors
	// NOTE - we are using one client per instance accessor.  This is not the most efficient
	// if there are resources sharing the same backends.  We assume there are only a small number
	// of resources in a collection.  For large pools of the same thing, we will implement a dedicated
	// pool controller.
	for _, access := range properties {
		err = access.Init(c.Scope(), c.LeaderFunc, c.options.PluginRetryInterval.Duration())
		if err != nil {
			return err
		}
	}

	c.properties = properties
	return
}
