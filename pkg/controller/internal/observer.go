package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

// InstanceObserver is an entity that observes an instance plugin
// for a list of instance descriptions at some specified polling interval.
// Not thread safe.
type InstanceObserver struct {
	instance.Plugin

	// Name is the name of the instance plugin
	Name plugin.Name `json:"plugin" yaml:"plugin"`

	// Select are the labels to use when querying for instances. This is the namespace.
	Select map[string]string

	// ObserveInterval is the polling interval for making an observation
	ObserveInterval types.Duration

	// CacheDescribeInstances is true to turn on caching with ttl equal to the ObserveInterval
	CacheDescribeInstances bool

	// KeySelector is a string template for selecting the join key from
	// an instance's instance.Description. This selector template should use escapes
	// so that the template {{ and }} are preserved.  For example,
	// SourceKeySelector: \{\{ .ID \}\}  # selects the ID field.
	KeySelector string

	// observations is a channel to receive a slice of current observations
	observations chan []instance.Description

	// lost is a channel to receive a slice of instances that have been lost in the current sample since last
	lost chan []instance.Description

	// observe is a function that returns the latest observations
	observe func() ([]instance.Description, error)

	// extractKey is a function that can extract the key from an instance.Description
	extractKey func(instance.Description) (string, error)

	poller *Poller
	ticker <-chan time.Time
	paused bool
	lock   sync.RWMutex
	ctx    context.Context
	cancel func()
}

var (
	minObserveInterval = 1 * time.Second
)

// Validate validates the receiver with default values provided if some optional fields are not set.
func (o *InstanceObserver) Validate(defaults *InstanceObserver) error {
	if defaults != nil {
		if err := mergo.Merge(o, defaults); err != nil {
			return err
		}
	}
	// critical checks
	if o.Name.Zero() {
		return fmt.Errorf("missing plugin name")
	}

	if o.ObserveInterval == 0 {
		return fmt.Errorf("observe interval not specified")
	}

	if o.KeySelector == "" {
		return fmt.Errorf("key selector not specified")
	}
	return nil
}

var (
	global     = map[plugin.Name]instance.Plugin{}
	globalLock sync.RWMutex
)

func cachedPlugin(scope scope.Scope, name plugin.Name, ttl, retry time.Duration) instance.Plugin {
	p := func() instance.Plugin {
		globalLock.RLock()
		defer globalLock.RUnlock()
		return global[name]
	}()

	if p == nil {

		instancePlugin := instance_plugin.LazyConnect(
			func() (instance.Plugin, error) {
				return scope.Instance(name.String())
			},
			retry)

		p = instance.CacheDescribeInstances(instancePlugin, ttl, time.Now)

		globalLock.Lock()
		global[name] = p
		globalLock.Unlock()

		log.Info("Allocated cached plugin", "key", name, "plugin", p)
		return p
	}
	log.Info("Using cached plugin", "key", name, "plugin", p)
	return p
}

// Init initializes the observer so that it can be started
func (o *InstanceObserver) Init(scope scope.Scope, retry time.Duration) error {

	o.lock.Lock()
	defer o.lock.Unlock()

	if retry == 0 {
		retry = 5 * time.Second
	}

	o.extractKey = KeyExtractor(o.KeySelector)

	// add caching layer
	if o.CacheDescribeInstances {
		o.Plugin = cachedPlugin(scope, o.Name, o.ObserveInterval.Duration(), retry)
	} else {
		o.Plugin = instance_plugin.LazyConnect(
			func() (instance.Plugin, error) {
				return scope.Instance(o.Name.String())
			},
			retry)
	}

	o.observe = func() ([]instance.Description, error) {

		// Because we are using caching, we want to cache the result set
		// where the individual instance name isn't part of the query.
		// This way, we can cache the entire result set and just reuse it
		// after doing filtering in-memory here.
		query := map[string]string{}
		instanceKey := ""
		if inst, has := o.Select[InstanceLabel]; has {
			instanceKey = inst
			for k, v := range o.Select {
				query[k] = v
			}
			delete(query, InstanceLabel) // don't query with the instance label
		} else {
			query = o.Select
		}

		desc, err := o.Plugin.DescribeInstances(query, true)
		if err != nil {
			return nil, err
		}

		// Because the result can be cached and a full set of instances,
		// we need to filter here to ensure the correct value
		found := []instance.Description{}
		if instanceKey != "" {
			for _, d := range desc {
				if v, has := d.Tags[InstanceLabel]; has && v == instanceKey {
					found = append(found, d)
				}
			}
		} else {
			found = desc
		}

		return found, err
	}

	o.observations = make(chan []instance.Description, 10)
	o.lost = make(chan []instance.Description, 10)

	last := []instance.Description{}

	o.ticker = time.Tick(o.ObserveInterval.AtLeast(minObserveInterval))
	o.poller = PollWithCleanup(
		// This determines if the action should be taken when time is up
		func() bool {
			o.lock.RLock()
			defer o.lock.RUnlock()

			return !o.paused
		},
		// This does the work
		func() (err error) {

			instances, err := o.observe()
			if err != nil {
				return err
			}

			log.Debug("polling", "V", debugV2,
				"plugin", o.Plugin,
				"select", o.Select,
				"observed", len(instances))

			// send the current observations
			select {
			case o.observations <- instances:
			default:
			}

			// send the lost instances
			lost := o.Difference(last, instances)

			select {
			case o.lost <- []instance.Description(lost):
			default:
			}

			last = instances
			return nil
		},
		o.ticker,
		func() {
			close(o.observations)
			log.Debug("observer poller stopped", "V", debugV2)
		})

	return nil
}

// KeyOf returns the key of the instance based on the key extractor configured here
func (o *InstanceObserver) KeyOf(instance instance.Description) (string, error) {
	return o.extractKey(instance)
}

// Start starts the observations. This call is nonblocking.
func (o *InstanceObserver) Start() {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.poller != nil {
		o.ctx, o.cancel = context.WithCancel(context.Background())
		go o.poller.Run(o.ctx)
	}
}

// Pause pauses or unpauses the observations
func (o *InstanceObserver) Pause(v bool) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.paused = v
}

// Stop permanents stops the observer
func (o *InstanceObserver) Stop() {
	o.lock.Lock()
	defer o.lock.Unlock()

	if o.poller != nil {
		o.poller.Stop()
		o.poller = nil
	}
}

// Observations returns the channel to receive observations.  When stopped, the channel is closed.
func (o *InstanceObserver) Observations() <-chan []instance.Description {
	return o.observations
}

// Lost returns the channel to receive instances that are missing in the current sample from the last.
func (o *InstanceObserver) Lost() <-chan []instance.Description {
	return o.lost
}

// Difference computes the difference before and after samples
func (o *InstanceObserver) Difference(before, after []instance.Description) instance.Descriptions {
	keyFunc := func(v instance.Description) (string, error) {
		return string(v.ID), nil
	}

	return instance.Difference(instance.Descriptions(before), keyFunc, instance.Descriptions(after), keyFunc)
}

// KeyExtractor returns a function that can extract the link key from an instance description
func KeyExtractor(text string) func(instance.Description) (string, error) {
	return func(i instance.Description) (string, error) {
		v, err := i.View(text)
		if err == nil {
			return v, nil
		}

		if i.LogicalID != nil {
			return string(*i.LogicalID), nil
		}
		return string(i.ID), nil
	}
}
