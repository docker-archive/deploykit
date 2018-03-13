package inventory

import (
	"context"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	inventory "github.com/docker/infrakit/pkg/controller/inventory/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

type resources map[string]instance.Description

type collection struct {
	*internal.Collection

	options inventory.Options
	model   *Model

	resources resources

	accessors map[string]*internal.InstanceAccess
	deleted   map[string]*internal.InstanceAccess

	cancel func()
}

var (
	// TopicFound is the topic for resource found
	TopicFound = types.PathFromString("ready")

	// TopicLost is the topic for resource lost
	TopicLost = types.PathFromString("lost")

	// TopicTerminate is the topic for starting to terminate the resource
	TopicTerminate = types.PathFromString("terminate")

	// TopicErr is the topic for errors
	TopicErr = types.PathFromString("error")
)

func newCollection(scope scope.Scope, options inventory.Options) (internal.Managed, error) {

	if err := mergo.Merge(&options, DefaultOptions); err != nil {
		return nil, err
	}

	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope,
		TopicFound,
		TopicLost,
		TopicErr,
	)
	if err != nil {
		return nil, err
	}
	c := &collection{
		Collection: base,
		options:    options,
		resources:  resources{},
		deleted:    map[string]*internal.InstanceAccess{},
	}
	// set the behaviors
	base.StartFunc = c.run
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec

	return c, nil
}

func (c *collection) updateSpec(spec types.Spec, previous *types.Spec) (err error) {

	prev := spec
	if previous != nil {
		prev = *previous
	}

	log.Debug("updateSpec", "spec", spec, "prev", prev)

	// parse input, then select the model to use
	properties := inventory.Properties{}
	err = spec.Properties.Decode(&properties)
	if err != nil {
		return
	}

	prevProperties := inventory.Properties{}
	err = prev.Properties.Decode(&prevProperties)
	if err != nil {
		return
	}

	options := c.options // the plugin options at initialization are the defaults
	err = spec.Options.Decode(&options)
	if err != nil {
		return
	}

	ctx := context.Background()

	if err = properties.Validate(ctx); err != nil {
		return
	}

	if err = options.Validate(ctx); err != nil {
		return
	}

	// NOTE - we are using one client per instance accessor.  This is not the most efficient
	// if there are resources sharing the same backends.

	accessors := map[string]*internal.InstanceAccess{}

	for name, accessList := range properties {

		for _, access := range accessList {
			copy := access

			err = c.configureAccessor(spec, name, &copy)
			if err != nil {
				return err
			}

			accessors[name] = &copy

			log.Debug("Initialized INCLUDED accessor", "name", name, "spec", spec, "access", access, "V", debugV2)
		}
	}

	deleted := map[string]*internal.InstanceAccess{}

	// For each in the previous spec that's not in the new spec, we need to start up the observation
	// so that we can detect whether there are real instances that needs to be terminated to match
	// the deletion in the new spec.
	for name, accessList := range prevProperties {

		for _, access := range accessList {
			if _, has := properties[name]; !has {

				// this is no longer in the newer version of the spec, so it's a deletion.
				// we need to have this still.

				copy := access

				if err := c.configureAccessor(prev, name, &copy); err != nil {
					return err
				}

				deleted[name] = &copy
				log.Debug("Initialize DELETED accessor", "name", name, "spec", spec, "access", access, "V", debugV2)
			}
		}
	}

	c.deleted = deleted

	// build the fsm model
	var model *Model
	model, err = BuildModel(properties, options)
	if err != nil {
		return
	}

	c.model = model
	c.accessors = accessors
	c.options = options
	return
}

func (c *collection) run(ctx context.Context) {

	// Start the model
	c.model.Start()

	// channels that aggregate from all the instance accessors
	type observation struct {
		name      string
		instances []instance.Description
	}

	accessors := map[string]*internal.InstanceAccess{}

	for n, a := range c.accessors {
		accessors[n] = a
	}
	for n, a := range c.deleted {
		accessors[n] = a
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	// Start all the instance accessors and wire up the observations.
	lostInstances := make(chan *observation, c.options.ChannelBufferSize)  // ch to aggregate all lost observations
	foundInstances := make(chan *observation, c.options.ChannelBufferSize) // ch to aggregate all found observations

	for k, a := range accessors {

		log.Debug("Set up events from instance accessor", "name", k, "V", debugV)
		go func(name string, accessor *internal.InstanceAccess) {

			for {
				select {
				case list, ok := <-accessor.Observations():
					if !ok {
						log.Debug("found observations done", "name", name, "V", debugV2)
						return
					}
					if len(list) > 0 {
						foundInstances <- &observation{name: name, instances: list}
						log.Debug("accessor found instances", "name", name, "count", len(list), "V", debugV)
					}
				case list, ok := <-accessor.Lost():
					if !ok {
						log.Debug("lost events done", "name", name, "V", debugV2)
						return
					}
					if len(list) > 0 {
						lostInstances <- &observation{name: name, instances: list}
						log.Debug("accessor lost instances", "name", name, "count", len(list), "V", debugV)
					}
				}
			}
		}(k, a)

		a.Start()
		log.Debug("accessor started", "key", k)
	}

	go func() {

	loop:
		for {

			select {

			case f, ok := <-c.model.Cleanup():
				if !ok {
					return
				}
				item := c.Collection.GetByFSM(f)
				if item != nil {
					c.Collection.Delete(item.Key)
				}
			case f, ok := <-c.model.Found():
				if !ok {
					return
				}
				item := c.Collection.GetByFSM(f)
				if item != nil {
					c.EventCh() <- event.Event{
						Topic:   c.Topic(TopicFound),
						Type:    event.Type("Found"),
						ID:      c.EventID(item.Key),
						Message: "resource found",
					}.Init()
				}

			case f, ok := <-c.model.Lost():
				if !ok {
					return
				}
				item := c.Collection.GetByFSM(f)
				if item != nil {
					c.EventCh() <- event.Event{
						Topic:   c.Topic(TopicLost),
						Type:    event.Type("Lost"),
						ID:      c.EventID(item.Key),
						Message: "resource lost",
					}.Init()
				}

			case f, ok := <-c.model.Destroy():
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {

					accessor := c.accessors[item.Key]
					log.Info("Destroy", "fsm", f.ID(), "item", item, "accessor", accessor)

					// !!!! This actually is *always* nil in the case where the accessor
					// section has been removed and we discover an instance that doesn't
					// correspond to anything.
					// The correct approach would be to use the *previous* version of the spec
					if accessor == nil {
						found, has := c.deleted[item.Key]
						if has {
							accessor = found
						}
					}

					if accessor == nil {
						log.Error("cannot find accessor for key", "key", item.Key)
						continue loop
					}

					// TODO - call instancePlugin.Destroy
					d := item.Data["instance"]
					if d == nil {
						log.Error("cannot find instance", "item", item.Key)
						continue loop
					}

					if dd, is := d.(instance.Description); is {

						log.Info("Destroy", "instanceID", dd.ID, "key", item.Key)
						err := accessor.Destroy(dd.ID, instance.Termination)
						log.Debug("destroy", "instanceID", dd.ID, "key", item.Key, "err", err)

						if err != nil {
							c.EventCh() <- event.Event{
								Topic:   c.Topic(TopicErr),
								Type:    event.Type("TerminateErr"),
								ID:      c.EventID(item.Key),
								Message: "destroying resource error",
							}.Init().WithError(err)
						} else {
							c.EventCh() <- event.Event{
								Topic:   c.Topic(TopicTerminate),
								Type:    event.Type("Terminate"),
								ID:      c.EventID(item.Key),
								Message: "destroyed resource",
							}.Init()
						}
					}
				}

			case lost, ok := <-lostInstances:
				if !ok {
					log.Info("Lost aggregator done")
					return
				}

				accessor, has := accessors[lost.name]
				if !has {
					log.Warn("cannot find accessor for lost instance", "name", lost.name)
					break
				}

				// Update the view in the metadata plugin
				c.MetadataGone(accessor.KeyOf, lost.instances)

				for _, n := range lost.instances {
					k, err := accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}

					if item := c.Collection.Get(k); item != nil {
						log.Error("lost", "instance", n, "name", lost.name, "key", k)
						item.State.Signal(resourceLost)
					}
					delete(c.resources, k)
				}

			case found, ok := <-foundInstances:
				if !ok {
					log.Info("Found aggregator done")
					return
				}

				accessor, has := accessors[found.name]
				if !has {
					log.Warn("cannot find accessor for found instance", "name", found.name)
					break
				}

				// Update the view in the metadata plugin
				export := []instance.Description{}

				for _, n := range found.instances {
					k, err := accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}
					item := c.Collection.Get(k)
					if item == nil {

						// In this case, the fsm isn't requested.. it's something we get out of band
						// that somehow shows up (or from previous runs but now the user has
						// removed it from the spec and performed a commit.
						f := c.model.New()
						item = c.Put(k, f, c.model.Spec(), map[string]interface{}{
							"instance": n,
						})

						log.Debug("New instance", "fsm", f, "instance", n, "V", debugV)

						export = append(export, n) // export to metadata

					} else {
						// if we already have entries stored, then see if the data changed
						prev := item.Data["instance"]
						if prev == nil {
							export = append(export, n)
						} else if dd, is := prev.(instance.Description); is {
							if dd.Fingerprint() != n.Fingerprint() {
								export = append(export, n)
							}
						}
					}

					c.resources[k] = n

					log.Debug("found", "instance", n, "name", found.name, "key", k, "V", debugV2)
					item.State.Signal(resourceFound)
					item.Data["instance"] = n
				}

				c.MetadataExport(accessor.KeyOf, export)
			}
		}
	}()

}

func (c *collection) stop() error {
	log.Info("stop")

	if c.model != nil {

		c.cancel()

		for k, accessor := range c.accessors {
			log.Debug("Stopping", "name", k, "V", debugV)
			accessor.Stop()
		}

		for k, accessor := range c.deleted {
			log.Debug("Stopping", "name", k, "V", debugV)
			accessor.Stop()
		}

		c.model.Stop()
		c.model = nil
	}
	return nil
}

func (c *collection) configureAccessor(spec types.Spec, name string, access *internal.InstanceAccess) error {
	if access.Labels == nil {
		access.Labels = map[string]string{}
	}

	err := access.InstanceObserver.Validate(DefaultAccessProperties)
	if err != nil {
		return err
	}

	return access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
}
