package internal

import (
	"context"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	instance_plugin "github.com/docker/infrakit/pkg/plugin/instance"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
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

// Init initializes the observer so that it can be started
func (o *InstanceObserver) Init(scope scope.Scope, leader func() stack.Leadership, retry time.Duration) error {

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

	o.observations = make(chan []instance.Description, 1)

	o.ticker = time.Tick(o.ObserveInterval.AtLeast(minObserveInterval))
	o.poller = PollWithCleanup(
		// This determines if the action should be taken when time is up
		func() bool {

			log.Debug("checking before poll", "V", debugV2)
			isLeader := false
			if leader != nil {
				v, err := leader().IsLeader()
				if err == nil {
					isLeader = v
				}
			}

			o.lock.RLock()
			defer o.lock.RUnlock()
			log.Debug("polling", "isLeader", isLeader, "V", debugV2, "freed", o.paused)
			return isLeader && !o.paused
		},
		// This does the work
		func() (err error) {

			instances, err := o.observe()
			if err != nil {
				return err
			}

			select {
			case o.observations <- instances:
			default:
			}

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

// Start starts the observations
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
	}
}

// Observations returns the channel to receive observations.  When stopped, the channel is closed.
func (o *InstanceObserver) Observations() <-chan []instance.Description {
	return o.observations
}

// Difference computes the difference before and after samples
func (o *InstanceObserver) Difference(before, after []instance.Description) instance.Descriptions {
	keyFunc := func(v instance.Description) (string, error) {
		return string(v.ID), nil
	}

	return instance.Difference(instance.Descriptions(before), keyFunc, instance.Descriptions(after), keyFunc)
}

// templateFrom returns a template after it has un-escaped any escape sequences
func templateFrom(source []byte) (*template.Template, error) {
	buff := template.Unescape(source)
	return template.NewTemplate(
		"str://"+string(buff),
		template.Options{MultiPass: false, MissingKey: template.MissingKeyError},
	)
}

// KeyExtractor returns a function that can extract the link key from an instance description
func KeyExtractor(text string) func(instance.Description) (string, error) {
	if text != "" {
		t, err := templateFrom([]byte(text))
		if err == nil {
			return func(i instance.Description) (view string, err error) {
				view, err = t.Render(i)
				return
			}
		}
	}

	return func(i instance.Description) (string, error) {
		if i.LogicalID != nil {
			return string(*i.LogicalID), nil
		}
		return string(i.ID), nil
	}
}
