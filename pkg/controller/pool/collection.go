package pool

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	pool "github.com/docker/infrakit/pkg/controller/pool/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

type resources map[string]instance.Description

type collection struct {
	*internal.Collection

	accessor *internal.InstanceAccess // current version
	last     *internal.InstanceAccess // last version

	spec types.Spec

	properties pool.Properties
	options    pool.Options

	model *Model

	resources resources

	cancel func()
}

var (
	// TopicProvision is the topic for provision
	TopicProvision = types.PathFromString("provision")

	// TopicProvisionErr is the topic for provision error
	TopicProvisionErr = types.PathFromString("error/provision")

	// TopicDestroy is the topic for destroy
	TopicDestroy = types.PathFromString("destroy")

	// TopicDestroyErr is the topic for destroy error
	TopicDestroyErr = types.PathFromString("error/destroy")

	// TopicPending is the topic for waiting for data
	TopicPending = types.PathFromString("pending")

	// TopicReady is the topic for resource ready
	TopicReady = types.PathFromString("ready")
)

func newCollection(scope scope.Scope, options pool.Options) (internal.Managed, error) {

	if err := mergo.Merge(&options, DefaultOptions); err != nil {
		return nil, err
	}

	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope,
		TopicProvision,
		TopicProvisionErr,
		TopicDestroy,
		TopicDestroyErr,
		TopicPending,
		TopicReady,
	)
	if err != nil {
		return nil, err
	}
	c := &collection{
		Collection: base,
		options:    options,
		resources:  resources{},
	}
	// set the behaviors
	base.StartFunc = c.run
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec
	base.TerminateFunc = c.terminate

	return c, nil
}

func (c *collection) updateSpec(spec types.Spec, previous *types.Spec) (err error) {

	prev := spec
	if previous != nil {
		prev = *previous
	}

	log.Debug("updateSpec", "spec", spec, "prev", prev)

	// parse input, then select the model to use
	properties := pool.Properties{}
	err = spec.Properties.Decode(&properties)
	if err != nil {
		return
	}

	prevProperties := pool.Properties{}
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

	log.Debug("Begin processing", "properties", properties, "previous", prevProperties, "options", options, "V", debugV2)

	err = c.configureAccessor(spec, &properties.InstanceAccess)
	if err != nil {
		return err
	}

	c.accessor = &properties.InstanceAccess

	log.Debug("Initialized current accessor", "spec", spec, "access", c.accessor, "V", debugV)

	err = c.configureAccessor(spec, &prevProperties.InstanceAccess)
	if err != nil {
		return err
	}
	c.last = &prevProperties.InstanceAccess

	log.Debug("Initialize last accessor", "previous", previous, "access", c.last, "V", debugV)

	// build the fsm model
	var model *Model
	model, err = BuildModel(properties, options)
	if err != nil {
		return
	}

	c.model = model
	c.spec = spec
	c.properties = properties
	c.options = options

	log.Debug("Starting with state", "properties", c.properties, "V", debugV)
	return
}

func (c *collection) run(ctx context.Context) {

	// Start the model
	c.model.Start()

	// channels that aggregate from all the instance accessors
	type observation struct {
		instances []instance.Description
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	// Start all the instance accessors and wire up the observations.

	lostInstances := make(chan *observation, c.options.ChannelBufferSize)  // ch to aggregate all lost observations
	foundInstances := make(chan *observation, c.options.ChannelBufferSize) // ch to aggregate all found observations

	log.Debug("Set up events from instance accessor", "V", debugV)
	go func(accessor *internal.InstanceAccess) {
		for {
			select {
			case list, ok := <-accessor.Observations():
				if !ok {
					log.Debug("found observations done", "V", debugV2)
					return
				}
				if len(list) > 0 {
					foundInstances <- &observation{instances: list}
					log.Debug("accessor found instances", "count", len(list), "V", debugV2)
				}
			case list, ok := <-accessor.Lost():
				if !ok {
					log.Debug("lost events done", "V", debugV2)
					return
				}
				if len(list) > 0 {
					lostInstances <- &observation{instances: list}
					log.Debug("accessor lost instances", "count", len(list), "V", debugV2)
				}
			}
		}
	}(c.accessor)

	c.accessor.Start()
	log.Debug("accessor started")

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
			case f, ok := <-c.model.Ready():
				if !ok {
					return
				}
				item := c.Collection.GetByFSM(f)
				if item != nil {
					c.EventCh() <- event.Event{
						Topic:   c.Topic(TopicReady),
						Type:    event.Type("Ready"),
						ID:      c.EventID(item.Key),
						Message: "resource ready",
					}.Init()
				}

			case f, ok := <-c.model.Pending():
				if !ok {
					return
				}
				item := c.Collection.GetByFSM(f)
				if item != nil {

					msg := fmt.Sprintf("%v : resource blocked waiting on dependencies", item.State)
					c.EventCh() <- event.Event{
						Topic:   c.Topic(TopicPending),
						Type:    event.Type("Pending"),
						ID:      c.EventID(item.Key),
						Message: msg,
					}.Init()
				}

			case f, ok := <-c.model.Destroy():
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {

					// We throttle provisioning based on the
					// parallelism parameter and the number of inflight
					// resources
					inTerminating := c.Collection.GetCountByState(terminating)

					if c.properties.Parallelism < inTerminating {
						// we need to send this back to the requested state
						item.State.Signal(throttle)
						continue
					}

					accessor := c.accessor
					log.Info("Destroy", "fsm", f.ID(), "item", item, "accessor", accessor)

					if accessor == nil {
						accessor = c.last
					}

					if accessor == nil {
						log.Error("cannot find accessor for seq", "seq", item.Key)
						continue loop
					}

					d := item.Data["instance"]
					if d == nil {
						log.Error("cannot find instance", "item", item.Key)
						continue loop
					}

					if dd, is := d.(instance.Description); is {

						// terminate asynchronously
						timer := time.NewTimer(c.options.DestroyDeadline.Duration())
						done := make(chan struct{})

						go func() {
							defer func() {
								e := recover()
								if e != nil {
									log.Error("Recovered from error while terminating", "err", e,
										"accessor", accessor,
										"instanceID", dd.ID, "item", item)
								}

								close(done)
							}()

							log.Info("Destroy", "instanceID", dd.ID, "key", item.Key)
							err := accessor.Destroy(dd.ID, instance.Termination)
							log.Debug("destroy", "instanceID", dd.ID, "key", item.Key, "err", err)

							if err != nil {

								log.Error("Cannot destroy", "err", err)
								item.State.Signal(terminateError)
								item.Error(err)

								c.EventCh() <- event.Event{
									Topic:   c.Topic(TopicDestroyErr),
									Type:    event.Type("DestroyErr"),
									ID:      c.EventID(item.Key),
									Message: "destroying resource error",
								}.Init().WithError(err)

							} else {

								c.EventCh() <- event.Event{
									Topic:   c.Topic(TopicDestroy),
									Type:    event.Type("Destroy"),
									ID:      c.EventID(item.Key),
									Message: "destroying resource",
								}.Init()

							}
						}()

						// Wait for the destroy to complete or when deadline is exceeded.
						select {
						case <-timer.C:
						case <-done:
						}
						timer.Stop()
					}
				}

			case f, ok := <-c.model.Provision():
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {

					// We throttle provisioning based on the
					// parallelism parameter and the number of inflight
					// resources
					inProvisioning := c.Collection.GetCountByState(provisioning)

					if c.properties.Parallelism < inProvisioning {
						// we need to send this back to the requested state
						item.State.Signal(throttle)
						continue
					}

					accessor := c.accessor
					spec, err := c.buildSpec(item, accessor.Spec)
					if err != nil {

						log.Error("Error building spec",
							"fsm", f.ID(), "item", item,
							"accessor", accessor, "spec", spec,
							"err", err)

						item.State.Signal(dependencyMissing)
						continue
					}

					// provision asynchronously
					timer := time.NewTimer(c.options.ProvisionDeadline.Duration())
					done := make(chan struct{})

					go func() {
						defer func() {
							e := recover()
							if e != nil {
								log.Error("Recovered from error while provisioning", "err", e,
									"accessor", accessor,
									"spec", spec, "item", item)
							}

							close(done)
						}()
						instanceID, err := accessor.Provision(spec)
						if err != nil {

							log.Error("Cannot provision", "err", err)
							item.State.Signal(provisionError)
							item.Error(err)

							c.EventCh() <- event.Event{
								Topic:   c.Topic(TopicProvisionErr),
								Type:    event.Type("ProvisionErr"),
								ID:      c.EventID(item.Key),
								Message: "error when provision",
							}.Init().WithError(err)

						} else {

							id := ""
							if instanceID != nil {
								id = string(*instanceID)
							}

							log.Info("Provisioned", "id", id, "spec", spec)

							/// don't do anything. next sample will make sure it moves to ready

							c.EventCh() <- event.Event{
								Topic:   c.Topic(TopicProvision),
								Type:    event.Type("Provision"),
								ID:      c.EventID(item.Key),
								Message: "provisioning resource",
							}.Init().WithDataMust(spec)
						}
					}()

					// Wait for the provision to complete or when deadline is exceeded.
					select {
					case <-timer.C:
					case <-done:
					}
					timer.Stop()

				}

			case lost, ok := <-lostInstances:
				if !ok {
					log.Info("Lost aggregator done")
					return
				}

				// Update the view in the metadata plugin
				c.MetadataGone(c.accessor.KeyOf, lost.instances)

				for _, n := range lost.instances {
					k, err := c.accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}

					if item := c.Collection.Get(k); item != nil {
						item.State.Signal(resourceLost)
						log.Warn("lost", "instance", n, "key", k)
					}
					delete(c.resources, k)
				}

			case found, ok := <-foundInstances:
				if !ok {
					log.Info("Found aggregator done")
					return
				}

				// Update the view in the metadata plugin
				export := []instance.Description{}

				for _, n := range found.instances {
					k, err := c.accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}
					item := c.Collection.Get(k)
					if item == nil {

						// In this case, the fsm isn't requested.. it's something we get out of band
						// that somehow shows up (or from previous runs but now the user has
						// removed it from the spec and performed a commit.
						f := c.model.Unmatched()
						item = c.Put(k, f, c.model.Spec(), map[string]interface{}{
							"instance": n,
						})

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

					log.Debug("found", "instance", n, "key", k, "V", debugV2)
					item.State.Signal(resourceFound)
					item.Data["instance"] = n
				}

				c.MetadataExport(c.accessor.KeyOf, export)
			}
		}
	}()

	log.Debug("Seeding instances", "count", c.properties.Count, "V", debugV)

	// Seed the initial fsm instances for the size of the collection
	for i := 0; i < c.properties.Count; i++ {

		f := c.model.Requested()
		k := fmt.Sprintf("%s_%04d", c.spec.Metadata.Name, i)

		item := c.Put(k, f, c.model.Spec(), nil)
		item.Ordinal = i

		log.Debug("requested", "id", f.ID(), "key", item.Key, "ordinal", item.Ordinal)
	}

	log.Debug("Seeded instances. Running.")
}

func (c *collection) terminate() error {
	c.Visit(func(item internal.Item) bool {
		item.State.Signal(terminate)
		return true
	})

	return nil
}

func (c *collection) stop() error {
	log.Info("stop")

	if c.model != nil {

		c.cancel()

		log.Debug("Stopping", "V", debugV)
		c.accessor.Stop()
		if c.last != nil {
			c.last.Stop()
		}
		c.model.Stop()
		c.model = nil
	}
	return nil
}

func (c *collection) configureAccessor(spec types.Spec, access *internal.InstanceAccess) error {
	if access.Select == nil {
		access.Select = map[string]string{}
	}
	access.Select[internal.CollectionLabel] = spec.Metadata.Name
	err := access.InstanceObserver.Validate(c.options.InstanceObserver)
	if err != nil {
		return err
	}

	return access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
}

func (c *collection) buildSpec(item *internal.Item, spec instance.Spec) (instance.Spec, error) {

	// Turn the spec into a blob and use that to parse the dependencies
	specAny, err := types.AnyValue(spec)
	if err != nil {
		return spec, err
	}

	evaled := types.EvalDepends(specAny,
		func(p types.Path) (interface{}, error) {
			v := types.Get(p, c.resources)
			return v, nil
		}) // should have all values populated

	specAny, err = types.AnyValue(evaled)
	if err != nil {
		return spec, err
	}
	if depends := types.ParseDepends(specAny); len(depends) > 0 {
		return spec, fmt.Errorf("missing data %v", specAny.String())
	}

	// Now take the whole thing and decode into a Spec
	processed := spec

	err = specAny.Decode(&processed)
	if err != nil {
		return spec, err
	}

	if processed.Tags == nil {
		processed.Tags = map[string]string{}
	}

	processed.Tags[internal.InstanceLabel] = item.Key
	processed.Tags[internal.CollectionLabel] = c.Collection.Spec.Metadata.Name
	processed.Tags[internal.SpecHash] = types.Fingerprint(specAny)
	types.NewLink().WriteMap(processed.Tags)

	// Additional labels in the InstanceAccess spec
	for k, v := range c.accessor.Select {
		processed.Tags[k] = v
	}

	return processed, nil
}
