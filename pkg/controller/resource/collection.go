package resource

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var (
	// DefaultProperties is the default properties for the controller
	DefaultProperties = resource.Properties{
		ModelProperties: defaultModelProperties,
	}

	// DefaultAccessProperties specifies some default parameters
	DefaultAccessProperties = &internal.InstanceObserver{
		ObserveInterval: types.Duration(1 * time.Second),
		KeySelector:     template.EscapeString(`{{.Tags.infrakit_resource_name}}`),
	}
)

type resources map[string]instance.Description

func (r resources) eval(p types.Path) (interface{}, error) {
	v := types.Get(p, r)
	if v == nil {
		return nil, fmt.Errorf("missing data")
	}
	return v, nil
}

type collection struct {
	*internal.Collection

	properties *resource.Properties
	options    resource.Options
	model      *Model

	resources resources

	watch    *Watch
	watching map[string]Watchers
	cancel   func()
}

func newCollection(scope scope.Scope, options resource.Options) (internal.Managed, error) {

	if err := options.Validate(context.Background()); err != nil {
		return nil, err
	}

	base, err := internal.NewCollection(scope)
	if err != nil {
		return nil, err
	}
	c := &collection{
		Collection: base,
		options:    options,
		watch:      &Watch{},
		resources:  resources{},
		watching:   map[string]Watchers{},
	}

	// set the behaviors
	base.StartFunc = c.run
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec
	return c, nil
}

func (c *collection) run(ctx context.Context) {

	// Start the model
	c.model.Start()

	// channels that aggregate from all the instance accessors
	type event struct {
		name      string
		instances []instance.Description
	}

	dependencyComplete := make(chan *event, len(c.properties.Resources))
	accessors := map[string]*internal.InstanceAccess(c.properties.Resources)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	// Start all the watchers that have any dependencies
	for k, w := range c.watching {
		ch := w.FanIn(ctx)
		go func(n string) {
			<-ch
			// send event we got dependency satisified
			dependencyComplete <- &event{name: n}
		}(k)
		log.Debug("aggregator", "key", k, "watch", w)
	}

	// Start all the instance accessors and wire up the events.

	lostInstances := make(chan *event, c.properties.ChannelBufferSize)  // ch to aggregate all lost events
	foundInstances := make(chan *event, c.properties.ChannelBufferSize) // ch to aggregate all found events

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
					foundInstances <- &event{name: name, instances: list}

				case list, ok := <-accessor.Lost():
					if !ok {
						log.Debug("lost events done", "name", name, "V", debugV)
						return
					}
					lostInstances <- &event{name: name, instances: list}
				}
			}
		}()

		// start
		accessor.Start()
		log.Debug("accessor started", "key", k)
	}

	go func() {

		for {

			select {

			case f, ok := <-c.model.Destroy():
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {
					accessor := c.properties.Resources[item.Key]
					log.Info("Destroy", "fsm", f.ID(), "item", item, "accessor", accessor)
				}

			case f, ok := <-c.model.Provision():
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {
					accessor := c.properties.Resources[item.Key]
					log.Info("Provision", "fsm", f.ID(), "item", item, "accessor", accessor)

					spec, err := c.populateDependencies(item.Key, accessor.Spec)
					if err != nil {
						item.State.Signal(dependencyMissing)
						continue
					}
					instanceID, err := accessor.Provision(spec)
					if err != nil {
						log.Error("cannot provision", "err", err)
						item.State.Signal(provisionError)

					} else {
						log.Info("provisioned", "id", instanceID)
						/// don't do anything. next sample will make sure it moves to ready
					}
				}

			case haveAllData, ok := <-dependencyComplete:
				if !ok {
					log.Info("All haveAllData done")
					return
				}
				// Signal that we have all dependencies met for a given object
				item := c.Collection.Get(haveAllData.name)
				if item != nil {
					log.Info("Has all dependencies", "name", haveAllData.name)
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
						log.Info("lost", "instance", n, "name", lost.name, "key", k)
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
				c.MetadataExport(accessor.KeyOf, found.instances)

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
					}

					c.resources[k] = n

					// Notify watchers if any
					c.watch.Notify(k)

					log.Info("found", "instance", n, "name", found.name, "key", k)
					item.State.Signal(resourceFound)
					item.Data["instance"] = n
				}
			}
		}
	}()

	log.Debug("Components running. About to seed instances")

	// Seed the initial fsm instances for each named resource in the config
	// For each accessor / resource we create one fsm
	for k := range c.properties.Resources {
		log.Debug("requesting", "key", k)
		f := c.model.Requested()
		c.Put(k, f, c.model.Spec(), nil)
		log.Debug("requested", "id", f.ID(), "key", k)
	}

	log.Debug("Seeded instances. Running.")
}

func (c *collection) stop() error {
	log.Info("stop")

	for k, accessor := range c.properties.Resources {
		log.Debug("Stopping", "name", k, "V", debugV)
		accessor.Stop()
	}

	c.model.Stop()
	return nil
}

func (c *collection) updateSpec(spec types.Spec) (err error) {

	defer log.Debug("updateSpec", "spec", spec, "err", err)

	// parse input, then select the model to use
	properties := DefaultProperties

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
	for _, access := range properties.Resources {

		err = access.InstanceObserver.Validate(DefaultAccessProperties)
		if err != nil {
			return err
		}

		err = access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
		if err != nil {
			return err
		}
	}

	watch, watching := processWatches(properties)
	log.Debug("watch/watching", "watch", watch, "watching", watching)

	// build the fsm model
	var model *Model
	model, err = BuildModel(properties)
	if err != nil {
		return
	}

	c.model = model
	c.watch = watch
	c.watching = watching
	c.properties = &properties
	return
}

func processWatches(properties resource.Properties) (watch *Watch, watching map[string]Watchers) {
	watch = &Watch{}
	watching = map[string]Watchers{}

	for key, access := range properties.Resources {
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

	properties, _ = dependV(properties, c.resources.eval) // should have all values populated
	any, err := types.AnyValue(properties)
	if err != nil {
		return spec, err
	}

	processed.Properties = any

	processed.Tags = map[string]string{
		ResourceNameLabel:       resourceName,
		ResourceCollectionLabel: c.Collection.Spec.Metadata.Name,
	}
	types.NewLink().WriteMap(processed.Tags)

	// Additional labels in the InstanceAccess spec
	access := c.properties.Resources[resourceName]
	if access != nil {
		for k, v := range access.Labels {
			processed.Tags[k] = v
		}
	}
	return processed, nil
}

const (
	// ResourceNameLabel is the label name used for labeling the resource with the name in the collection
	ResourceNameLabel = "infrakit_resource_name"
	// ResourceCollectionLabel is the the label used to label the name of the collection
	ResourceCollectionLabel = "infrakit_resource_collection"
)

func dependV(v interface{}, fetcher func(types.Path) (interface{}, error)) (interface{}, bool) {
	substituted := false
	switch v := v.(type) {
	case map[string]interface{}:
		for k, vv := range v {
			newV, substitute := dependV(vv, fetcher)
			if substitute {
				v[k] = newV
				substituted = true
			}
		}
	case []interface{}:
		for i, vv := range v {
			newV, substitute := dependV(vv, fetcher)
			if substitute {
				v[i] = newV
				substituted = true
			}
		}
	case string:
		if p, ok := parseDepends(v); ok {
			// found a depend, now get the real value and swap
			newV, err := fetcher(p)
			if err != nil {
				return err, true // return an error attached at the same location
			}
			return newV, true
		}
	default:
	}
	return v, substituted
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
