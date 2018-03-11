package resource

import (
	"context"
	"fmt"
	"regexp"
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
	options   resource.Options
	model     *Model

	resources resources

	deleted map[string]*internal.InstanceAccess

	watch    *Watch
	watching map[string]Watchers
	cancel   func()
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
		Collection: base,
		options:    options,
		watch:      &Watch{},
		resources:  resources{},
		watching:   map[string]Watchers{},
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

		err = c.configureAccessor(spec, name, &copy)
		if err != nil {
			return err
		}

		accessors[name] = &copy

		log.Debug("Initialized INCLUDED accessor", "name", name, "spec", spec, "access", accessors[name], "V", debugV)
	}

	log.Debug("Process watches / dependencies")
	watch, watching := processWatches(properties)
	log.Debug("watch/watching", "watch", watch, "watching", watching)

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

			if err := c.configureAccessor(prev, name, &copy); err != nil {
				return err
			}

			deleted[name] = &copy
			log.Debug("Initialize DELETED accessor", "name", name, "spec", spec, "access", deleted[name], "V", debugV)
		}
	}
	c.deleted = deleted

	// build the fsm model
	var model *Model
	model, err = BuildModel(properties, options)
	if err != nil {
		return
	}

	c.accessors = accessors
	c.model = model
	c.options = options

	c.watch = watch
	c.watching = watching

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

	dependencyComplete := make(chan *observation, len(c.accessors))
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
	for k, w := range c.watching {
		ch := w.FanIn(ctx)
		go func(n string) {
			<-ch
			// send event we got dependency satisified
			dependencyComplete <- &observation{name: n}
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
					c.EventCh() <- event.Event{
						Topic:   c.Topic(TopicPending),
						Type:    event.Type("Pending"),
						ID:      c.EventID(item.Key),
						Message: "resource blocked waiting on dependencies",
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
								Message: "destroyed resource",
							}.Init()
						}
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

					func() {
						defer func() {
							e := recover()
							if e != nil {
								log.Error("Recovered from error while provisioning", "err", e,
									"accessor", accessor,
									"spec", spec, "item", item)
							}
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
				}

			case haveAllData, ok := <-dependencyComplete:
				if !ok {
					log.Info("All haveAllData done")
					return
				}
				// Signal that we have all dependencies met for a given object
				item := c.Collection.Get(haveAllData.name)
				if item != nil {
					log.Debug("Met all dependencies", "name", haveAllData.name, "V", debugV)
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
					c.watch.Notify(k)

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

	// inject a filter specifically for *this* resource
	access.Labels[internal.CollectionLabel] = spec.Metadata.Name
	access.Labels[internal.InstanceLabel] = name

	err := access.InstanceObserver.Validate(DefaultAccessProperties)
	if err != nil {
		return err
	}

	return access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
}
func processWatches(properties resource.Properties) (watch *Watch, watching map[string]Watchers) {
	watch = &Watch{}
	watching = map[string]Watchers{}

	for key, access := range properties {
		watchers := Watchers{}

		// get a list of dependencies from the Spec properties
		for _, path := range depends(access.Spec.Properties) {

			key, err := keyFromPath(path)
			if err != nil {
				log.Error("bad dep path", "err", err)
				continue
			}

			w := make(chan struct{})
			watchers = append(watchers, w)
			watch.Add(key, w)
		}

		watching[key] = watchers
	}
	return
}

// Assumption: the spec.Properties is fully rendered.  We can take the spec.Properties and
// generate a list of dependencies via depends().  Now we are rendering this spec.Properties
// into the final form with all the dependencies substituted.
func (c *collection) populateDependencies(resourceName string, spec instance.Spec) (instance.Spec, error) {

	processed := spec
	var properties interface{}
	err := types.Decode(processed.Properties.Bytes(), &properties)
	if err != nil {
		return spec, err
	}

	properties = dependV(properties,
		func(p types.Path) (interface{}, error) {
			v := types.Get(p, c.resources)
			return v, nil
		}) // should have all values populated

	any, err := types.AnyValue(properties)
	if err != nil {
		return spec, err
	}

	if depends := depends(any); len(depends) > 0 {
		return spec, fmt.Errorf("missing data %v", any.String())
	}

	processed.Properties = any

	processed.Tags = map[string]string{
		internal.InstanceLabel:   resourceName,
		internal.CollectionLabel: c.Collection.Spec.Metadata.Name,
	}
	types.NewLink().WriteMap(processed.Tags)

	// Additional labels in the InstanceAccess spec
	access := c.accessors[resourceName]
	if access != nil {
		for k, v := range access.Labels {
			processed.Tags[k] = v
		}
	}

	return processed, nil
}

func dependV(v interface{}, fetcher func(types.Path) (interface{}, error)) interface{} {
	switch v := v.(type) {
	case map[string]interface{}:
		for k, vv := range v {
			v[k] = dependV(vv, fetcher)
		}
	case []interface{}:
		for i, vv := range v {
			v[i] = dependV(vv, fetcher)
		}
	case string:
		if p, ok := parseDepends(v); ok {
			// found a depend, now get the real value and swap
			newV, err := fetcher(p)
			if err != nil {
				return err.Error()
			}
			if newV != nil {
				return newV
			}
		}
	default:
	}
	return v
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

// depends parses the blob and returns a list of paths. The path's first component is the
// name of the resource. e.g. dep `net1/cidr`
func depends(any *types.Any) []types.Path {
	var v interface{}
	err := any.Decode(&v)
	if err != nil {
		return nil
	}
	l := parse(v, []types.Path{})
	types.SortPaths(l)
	return l
}

// Special format of a string value to denote a dependency on another resource's (within the same collection)
// property field.  Eg. "@depends('net1/cidr')@"
var dependsRegex = regexp.MustCompile("\\@depend\\('(([[:alnum:]]|-|_|\\.|/|\\[|\\])+)'\\)\\@")

func parse(v interface{}, found []types.Path) (out []types.Path) {
	switch v := v.(type) {
	case map[string]interface{}:
		for _, vv := range v {
			out = append(out, parse(vv, nil)...)
		}
	case []interface{}:
		for _, vv := range v {
			out = append(out, parse(vv, nil)...)
		}
	case string:
		if p, ok := parseDepends(v); ok {
			out = append(out, p)
		}
	default:
	}
	out = append(found, out...)
	return
}

func parseDepends(text string) (types.Path, bool) {
	matches := dependsRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return types.PathFromString(matches[1]), true
	}
	return types.Path{}, false
}
