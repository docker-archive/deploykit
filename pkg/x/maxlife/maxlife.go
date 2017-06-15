package maxlife

import (
	"math"
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
	stop    chan struct{}
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

// Start starts the controller running.  This call does not block.
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

	tick := time.Tick(c.poll)

loop:
	for {

		select {

		case now := <-tick:

			log.Info("TICK")

			described, err := c.plugin.DescribeInstances(c.tags, false)
			if err != nil {
				// Transient error?
				log.Warn("error describing instances", "name", c.name, "err", err)
				continue
			}

			// If we are not in a steady state, don't destroy the instances.  This is
			// important so that we don't take down the whole cluster without restraint.
			if len(described) != initialCount {
				log.Info("Not steady state yet. No action")
				continue
			}

			// Just pick a single oldest instance per polling cycle.  This is so
			// that we don't end up destroying the cluster by taking down too many instances
			// all at once.
			oldest := maxAge(described, now)

			// check to make sure the age is over the maxlife
			if age(oldest, now) > c.maxlife {

				log.Info("Destroying", "oldest", oldest, "age", age(oldest, now), "maxlife", c.maxlife)

				// terminate it and hope the group controller restores with a new intance
				err = c.plugin.Destroy(oldest.ID, instance.Termination)
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

// age returns the age to the nearest second
func age(instance instance.Description, now time.Time) (age time.Duration) {
	link := types.NewLinkFromMap(instance.Tags)
	if link.Valid() {
		age = now.Sub(link.Created())
		age = time.Duration(math.Floor(age.Seconds())) * time.Second
	}
	return
}

func maxAge(instances []instance.Description, now time.Time) (result instance.Description) {
	if len(instances) == 0 || instances == nil {
		return
	}

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
	result = instances[found]
	return
}
