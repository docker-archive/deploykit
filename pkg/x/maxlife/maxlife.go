package maxlife

import (
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "x/maxlife")

// Controller is a single maxlife controller that works with a single instance
// plugin to ensure that the resource instances managed by the plugin do not
// exceed a specified lifetime.
type Controller struct {
	name    string
	plugin  instance.Plugin
	poll    time.Duration
	maxlife time.Duration
	tags    map[string]string

	stop chan struct{}
}

// NewController creates a controller based on the given plugin and configurations.
func NewController(name string, plugin instance.Plugin, poll, maxlife time.Duration,
	tags map[string]string) *Controller {
	return &Controller{
		name:    name,
		plugin:  plugin,
		stop:    make(chan struct{}),
		poll:    poll,
		maxlife: maxlife,
	}
}

// Stop stops the controller
func (c *Controller) Stop() {
	close(c.stop)
}

func (c *Controller) Start() {
	go c.run()
}

func (c *Controller) run() {
	initialCount := 0
loop:
	for {
		described, err := c.plugin.DescribeInstances(c.tags, false)
		if err != nil {
			log.Warn("cannot get initial count", "name", c.name, "err", err)
		} else {
			initialCount = len(described)
			break loop
		}

		// Wait a little bit before trying again -- use the same poll interval
		<-time.After(c.poll)
	}

	// Now we have initial state, continue with the sampling and monitoring of instances.
	c.ensureMaxlife(initialCount)
}

func (c *Controller) ensureMaxlife(initialCount int) {

	// Count is used to track the steady state...  we don't want to keep killing instances
	// if the counts are steadily decreasing.  The idea here is that once we terminate a resource
	// another one will be resurrected so we will be back to steady state.
	// Of course it's possible that the size of the cluster actually is decreased.  So we'd
	// wait for a few samples to get to steady state before we terminate another instance.
	// Currently we assume damping == 1 or 1 successive samples of delta >= 0 is sufficient to terminate
	// another instance.

	last := initialCount
	tick := time.Tick(c.poll)
loop:
	for {

		select {

		case now := <-tick:

			described, err := c.plugin.DescribeInstances(c.tags, false)
			if err != nil {
				// Transient error?
				log.Warn("error describing instances", "name", c.name, "err", err)
				continue
			}

			// TODO -- we should compute the 2nd derivative wrt time to make sure we
			// are truly in a steady state...

			current := len(described)
			delta := current - last
			last = current

			if current < 2 {
				log.Info("there are less than 2 instances.  No actions.", "name", c.name)
				continue
			}

			if delta < 0 {
				// Don't do anything if there are fewer instances at this iteration
				// than the last.  We want to wait until steady state
				log.Info("fewer instances in this round.  No actions taken", "name", c.name)
				continue
			}

			// Just pick a single oldest instance per polling cycle.  This is so
			// that we don't end up destroying the cluster by taking down too many instances
			// all at once.
			oldest := maxAge(described, now)

			// check to make sure the age is over the maxlife
			if age(oldest, now) > c.maxlife {
				// terminate it and hope the group controller restores with a new intance
				err = c.plugin.Destroy(oldest.ID)
				if err != nil {
					log.Warn("cannot destroy instance", "name", c.name, "id", oldest.ID, "err", err)
					continue
				}
			}

		case <-c.stop:
			log.Info("stop requested", "name", c.name)
			break loop
		}
	}

	log.Info("maxlife stopped", "name", c.name)
	return
}

func age(instance instance.Description, now time.Time) (age time.Duration) {
	link := types.NewLinkFromMap(instance.Tags)
	if link.Valid() {
		age = now.Sub(link.Created())
	}
	return
}

func maxAge(instances []instance.Description, now time.Time) instance.Description {
	// check to see if the tags of the instances have links.  Links have a creation date and
	// we can use it to compute the age
	var max time.Duration
	var found = 0
	for i, instance := range instances {
		age := age(instance, now)
		if age > max {
			max = age
			found = i
		}
	}
	return instances[found]
}
