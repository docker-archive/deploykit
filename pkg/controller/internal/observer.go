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

	// Plugin is the name of the instance plugin
	Plugin plugin.Name

	// Labels are the labels to use when querying for instances. This is the namespace.
	Labels map[string]string

	// ObserveInterval is the polling interval for making an observation
	ObserveInterval types.Duration

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
	if o.Plugin.Zero() {
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

// Init initializes the observer so that it can be started
func (o *InstanceObserver) Init(scope scope.Scope, retry time.Duration) error {

	o.lock.Lock()
	defer o.lock.Unlock()

	if retry == 0 {
		retry = 5 * time.Second
	}

	o.extractKey = KeyExtractor(o.KeySelector)

	instancePlugin := instance_plugin.LazyConnect(
		func() (instance.Plugin, error) {
			return scope.Instance(o.Plugin.String())
		},
		retry)

	o.observe = func() ([]instance.Description, error) {
		return instancePlugin.DescribeInstances(o.Labels, true)
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
				"freed", o.paused,
				"plugin", o.Plugin,
				"labels", o.Labels,
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
