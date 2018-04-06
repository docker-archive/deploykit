package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

type resources map[string]instance.Description

type collection struct {
	*internal.Collection

	accessors map[string]*internal.InstanceAccess

	properties resource.Properties
	options    resource.Options
	model      *Model

	resources resources

	deleted map[string]*internal.InstanceAccess

	provisionWatch    *Watch
	provisionWatching map[string]Watchers
	destroyWatch      *Watch
	destroyWatching   map[string]Watchers
	cancel            func()
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

func newCollection(scope scope.Scope, options resource.Options) (internal.Managed, error) {

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
		Collection:        base,
		options:           options,
		provisionWatch:    &Watch{},
		provisionWatching: map[string]Watchers{},
		destroyWatch:      &Watch{},
		destroyWatching:   map[string]Watchers{},
		resources:         resources{},
		deleted:           map[string]*internal.InstanceAccess{},
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
	properties := resource.Properties{}
	err = spec.Properties.Decode(&properties)
	if err != nil {
		return
	}

	prevProperties := resource.Properties{}
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

	// NOTE - we are using one client per instance accessor.  This is not the most efficient
	// if there are resources sharing the same backends.  We assume there are only a small number
	// of resources in a collection.  For large pools of the same thing, we will implement a dedicated
	// pool controller.

	accessors := map[string]*internal.InstanceAccess{}

	for name, access := range properties {

		copy := access

		// merge defaults
		mergo.Merge(&copy, internal.InstanceAccess{
			InstanceObserver: options.InstanceObserver,
		})

		err = c.configureAccessor(spec, name, &copy)
		if err != nil {
			return err
		}

		accessors[name] = &copy

		log.Debug("Initialized INCLUDED accessor", "name", name, "spec", spec, "access", accessors[name], "V", debugV)
	}

	// Handle deletion

	deleted := map[string]*internal.InstanceAccess{}

	// For each in the previous spec that's not in the new spec, we need to start up the observation
	// so that we can detect whether there are real instances that needs to be terminated to match
	// the deletion in the new spec.
	for name, access := range prevProperties {

		if _, has := properties[name]; !has {

			// this is no longer in the newer version of the spec, so it's a deletion.
			// we need to have this still.

			copy := access

			// merge defaults
			mergo.Merge(&copy, internal.InstanceAccess{
				InstanceObserver: options.InstanceObserver,
			})

			if err := c.configureAccessor(prev, name, &copy); err != nil {
				return err
			}

			deleted[name] = &copy
			log.Debug("Initialize DELETED accessor", "name", name, "spec", spec, "access", deleted[name], "V", debugV)
		}
	}
	c.deleted = deleted

	log.Debug("Process provisioning watches / dependencies")
	provisionWatch, provisionWatching := processProvisionWatches(properties)
	log.Debug("provisionWatch/provsionWatching", "watch", provisionWatch, "watching", provisionWatching)

	log.Debug("Process destroy watches / dependencies")
	destroyWatch, destroyWatching := processDestroyWatches(properties)
	log.Debug("destroynWatch/destroyWatching", "watch", destroyWatch, "watching", destroyWatching)

	// build the fsm model
	var model *Model
	model, err = BuildModel(properties, options)
	if err != nil {
		return
	}

	c.accessors = accessors
	c.model = model
	c.properties = properties
	c.options = options

	c.provisionWatch = provisionWatch
	c.provisionWatching = provisionWatching
	c.destroyWatch = destroyWatch
	c.destroyWatching = destroyWatching

	return
}

func (c *collection) run(ctx context.Context) {

	for k, v := range c.accessors {
		log.Debug("Running with accessors", "key", k, "accessor", v, "V", debugV2)
	}

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

	// Start all the watchers that have any dependencies
	provisionDependencyComplete := make(chan *observation, len(c.accessors))
	for k, w := range c.provisionWatching {
		ch := w.FanIn(ctx)
		go func(n string) {
			<-ch
			// send event we got dependency satisified
			provisionDependencyComplete <- &observation{name: n}
		}(k)
		log.Debug("aggregator", "key", k, "watch", w)
	}

	// Start all the watchers that have any dependencies
	destroyDependencyComplete := make(chan *observation, len(c.accessors))
	for k, w := range c.destroyWatching {
		ch := w.FanIn(ctx)
		go func(n string) {
			<-ch
			// send event we got dependency satisified
			destroyDependencyComplete <- &observation{name: n}
		}(k)
		log.Debug("aggregator", "key", k, "watch", w)
	}

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
						log.Debug("accessor found instances", "name", name, "count", len(list), "V", debugV2)
					}
				case list, ok := <-accessor.Lost():
					if !ok {
						log.Debug("lost events done", "name", name, "V", debugV2)
						return
					}
					if len(list) > 0 {
						lostInstances <- &observation{name: name, instances: list}
						log.Debug("accessor lost instances", "name", name, "count", len(list), "V", debugV2)
					}
				}
			}
		}(k, a)

		a.Start()
		log.Debug("accessor started", "key", k)
	}

	go func() {

		okToDestroy := map[string]int{}

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

					accessor := c.accessors[item.Key]
					log.Info("Destroy", "fsm", f.ID(), "item", item, "accessor", accessor,
						"watch", c.destroyWatch, "watching", c.destroyWatching)

					// Check to see if we are clear to destroy.
					// Generally, okToDestroy is the easiest to check since it's updated with
					// the key of the resource that has met all dependencies for destroy; however,
					// for all the first resources that don't depend on anything else, the okToDestroy
					// would never be populated because there are no dependencies that needs to be met
					// in the first place.  So we need to check to see if the item is watching anything else.
					// If the item is not watching anything at this time, then it's safe to destroy.
					if _, has := okToDestroy[item.Key]; !has {

						if watchers, deps := c.destroyWatching[item.Key]; deps {

							log.Error("cannot destroy because of dependencies", "item", item.Key, "watchers", len(watchers))
							item.State.Signal(dependencyMissing)

							continue
						}
					}

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
					accessor := c.accessors[item.Key]

					spec, err := c.populateDependencies(item.Key, accessor.Spec)
					if err != nil {

						log.Error("Dependency missing",
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

			case haveAllData, ok := <-provisionDependencyComplete:
				if !ok {
					log.Info("Provision haveAllData done")
					return
				}
				// Signal that we have all dependencies met for a given object
				item := c.Collection.Get(haveAllData.name)
				if item != nil {
					log.Debug("Provision: met all dependencies", "name", haveAllData.name, "V", debugV)
					item.State.Signal(dependencyReady)
				}

			case haveAllData, ok := <-destroyDependencyComplete:
				if !ok {
					log.Info("Destroy haveAllData done")
					return
				}
				// Signal that we have all dependencies met for a given object
				item := c.Collection.Get(haveAllData.name)
				if item != nil {

					okToDestroy[item.Key] = 1

					log.Debug("Destroy : met all dependencies", "name", haveAllData.name, "V", debugV)
					item.State.Signal(dependencyReady)

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
						item.State.Signal(resourceLost)
						log.Error("lost", "instance", n, "name", lost.name, "key", k)

						// Notify watchers if any
						c.destroyWatch.Notify(k)
						log.Debug("lost notified destroyWatch", "instance", n, "name", lost.name, "key", k, "V", debugV2)
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

					// Notify watchers if any
					c.provisionWatch.Notify(k)

					log.Debug("found", "instance", n, "name", found.name, "key", k, "V", debugV2)
					item.State.Signal(resourceFound)
					item.Data["instance"] = n
				}

				c.MetadataExport(accessor.KeyOf, export)
			}
		}
	}()

	log.Debug("Seeding instances")

	// Seed the initial fsm instances for each named resource in the config
	// For each accessor / resource we create one fsm
	for k := range c.accessors {
		log.Debug("requesting", "key", k)
		f := c.model.Requested()
		c.Put(k, f, c.model.Spec(), nil)
		log.Debug("requested", "id", f.ID(), "key", k)
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
	if access.Select == nil {
		access.Select = map[string]string{}
	}

	// inject a filter specifically for *this* resource
	access.Select[internal.CollectionLabel] = spec.Metadata.Name
	access.Select[internal.InstanceLabel] = name

	err := access.InstanceObserver.Validate(c.options.InstanceObserver)
	if err != nil {
		return err
	}

	return access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
}

func keyFromPath(path types.Path) (key string, err error) {
	k := path.Clean().Index(0)
	if k == nil {
		err = fmt.Errorf("no key %v", path)
		return
	}
	key = *k
	return
}

func processProvisionWatches(properties resource.Properties) (watch *Watch, watching map[string]Watchers) {
	watch = &Watch{}
	watching = map[string]Watchers{}

	for key, access := range properties {
		watchers := Watchers{}

		// get a list of dependencies from the Spec properties
		any, err := types.AnyValue(access.Spec)
		if err != nil {
			log.Error("Error parsing spec", "spec", access.Spec, "err", err)
			continue
		}

		for _, path := range types.ParseDepends(any) {

			dependedOnKey, err := keyFromPath(path)
			if err != nil {
				log.Error("bad dep path", "err", err)
				continue
			}

			// We only allow a modifier called post-provision: when we consider the dependencies.
			// This is so that provisioning of a resource can be gated by the post-provisioning step / data of another.
			// For all practical purposes, this is like a totally different key the object will watch.  If this is never
			// fulfilled, the watcher will be blocked forever from being provisioned.
			if i := strings.Index(dependedOnKey, ":"); i > 0 && dependedOnKey[0:i] != "post-provision" {
				continue
			}

			w := make(chan struct{})
			watchers = append(watchers, w)
			watch.Add(dependedOnKey, w)
		}

		watching[key] = watchers
	}
	return
}

func processDestroyWatches(properties resource.Properties) (watch *Watch, watching map[string]Watchers) {
	watch = &Watch{}
	watching = map[string]Watchers{}

	for key, access := range properties {

		// Get a list of dependencies from the Spec properties
		// For an item X this returns a list of items X depends ON.

		// get a list of dependencies from the Spec properties, including init
		any, err := types.AnyValue(access.Spec)
		if err != nil {
			log.Error("Error parsing spec", "spec", access.Spec, "err", err)
			continue
		}
		for _, path := range types.ParseDepends(any) {

			dependedOnKey, err := keyFromPath(path)
			if err != nil {
				log.Error("bad dep path", "err", err)
				continue
			}

			// Disallow any modifiers because we don't care about modifiers like post-provision (which is
			// not a valid dependency for termination of this resource.
			if strings.Index(dependedOnKey, ":") > 0 {
				continue
			}

			_, has := watching[dependedOnKey]
			if !has {
				watching[dependedOnKey] = Watchers{}
			}

			// add THIS (X) as the object that the depended on object is watching
			w := make(chan struct{})
			watching[dependedOnKey] = append(watching[dependedOnKey], w)

			watch.Add(key, w)
		}

	}
	return
}

// Assumption: the spec.Properties is fully rendered.  We can take the spec.Properties and
// generate a list of dependencies via depends().  Now we are rendering this spec.Properties
// into the final form with all the dependencies substituted.
func (c *collection) populateDependencies(resourceName string, spec instance.Spec) (instance.Spec, error) {

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

	processed.Tags[internal.InstanceLabel] = resourceName
	processed.Tags[internal.CollectionLabel] = c.Collection.Spec.Metadata.Name
	processed.Tags[internal.SpecHash] = types.Fingerprint(specAny)
	types.NewLink().WriteMap(processed.Tags)

	// Additional labels in the InstanceAccess spec
	access := c.accessors[resourceName]
	if access != nil {
		for k, v := range access.Select {
			processed.Tags[k] = v
		}
	}

	return processed, nil
}
