package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/controller/internal"
	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type collection struct {
	*internal.Collection

	properties resource.Properties
	options    resource.Options
	model      resource.Model

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
		watching:   map[string]Watchers{},
	}

	// set the behaviors
	base.StartFunc = c.run
	base.StopFunc = c.stop
	base.UpdateSpecFunc = c.updateSpec
	return c, nil
}

func (c *collection) run(ctx context.Context) {

	// channels that aggregate from all the instance accessors
	type event struct {
		name      string
		instances []instance.Description
	}

	allLost := make(chan *event, c.options.LostBufferSize)
	allFound := make(chan *event, c.options.FoundBufferSize)
	allDependsMet := make(chan *event, len(c.properties.Resources))

	accessors := map[string]*internal.InstanceAccess(c.properties.Resources)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	// Start the model
	c.model.Start()

	// For each accessor / resource we create one fsm
	for k := range c.properties.Resources {
		f := c.model.Requested()
		c.Put(k, f, c.model.Spec(), nil)
	}

	destroyCh := c.model.Destroy()
	provisionCh := c.model.Provision()

	go func() {
		for {
			select {
			case f, ok := <-destroyCh:
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {
					accessor := c.properties.Resources[item.Key]
					log.Info("Destroy", "fsm", f.ID(), "item", item, "accessor", accessor)
				}

			case f, ok := <-provisionCh:
				if !ok {
					return
				}

				item := c.Collection.GetByFSM(f)
				if item != nil {
					accessor := c.properties.Resources[item.Key]
					log.Info("Provision", "fsm", f.ID(), "item", item, "accessor", accessor)

					instanceId, err := accessor.Provision(c.populateDependencies)
					if err != nil {
						log.Error("cannot provision", "err", err)
					} else {
						log.Info("provisioned", "id", instanceId)
					}
				}
			}
		}
	}()

	// Start all the watchers that have any dependencies
	for k, w := range c.watching {
		ch := w.FanIn(ctx)
		go func(n string) {
			<-ch
			// send event we got dependency satisified
			allDependsMet <- &event{name: n}
		}(k)
	}

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

	type entry struct {
		Key        string
		Properties interface{}
	}

	go func() {

		for {

			select {

			case depends, ok := <-allDependsMet:
				if !ok {
					log.Info("All depends done")
					return
				}
				// Signal that we have all dependencies met for a given object
				item := c.Collection.Get(depends.name)
				if item != nil {
					log.Info("Has all dependencies", "name", depends.name)
					c.model.Found() <- item.State.FSM
				}

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

				// Update the view in the metadata plugin
				c.MetadataRemove(accessor.KeyOf, lost.instances)

				for _, n := range lost.instances {
					k, err := accessor.KeyOf(n)
					if err != nil {
						log.Error("error getting key", "err", err, "instance", n)
						break
					}

					if item := c.Collection.Get(k); item != nil {
						log.Info("lost", "instance", n, "name", lost.name, "key", k)
						c.model.Lost() <- item.State.FSM
					}

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

					// Notify watchers if any
					c.watch.Notify(k)

					log.Info("found", "instance", n, "name", found.name, "key", k)
					c.model.Found() <- item.State.FSM
				}
			}
		}
	}()
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
	for _, access := range properties.Resources {
		err = access.Init(c.Scope(), c.options.PluginRetryInterval.AtLeast(1*time.Second))
		if err != nil {
			return err
		}
	}

	watch := &Watch{}
	watching := map[string]Watchers{}

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

	// build the fsm model
	var model resource.Model
	model, err = BuildModel(properties)
	if err != nil {
		return
	}

	c.model = model
	c.watch = watch
	c.watching = watching
	c.properties = properties
	return
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

func (c *collection) populateDependencies(spec instance.Spec) (instance.Spec, error) {
	return spec, nil
}

// depends parses the blob and returns a list of paths
// examples)  dep `./net1/cidr` , dep `mystack/resource/networking/net2`
func depends(any *types.Any) []types.Path {
	return nil
}
