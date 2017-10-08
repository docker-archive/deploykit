package instance

import (
	"time"

	"github.com/deckarep/golang-set"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "util/event/instance")

const (
	eventType = event.Type("tracker")
)

// NewTracker returns an event plugin implementation that can generate events based on resources it trackers
func NewTracker(name string, plugin instance.Plugin, tick <-chan time.Time, tags map[string]string) *Tracker {
	m := &Tracker{
		poll:   tick,
		Plugin: plugin,
		Name:   name,
		tags:   tags,
	}
	return m.init()
}

// Tracker implements the event spi -- just just calls the Describe to get a
// list of known instances, and report anything it hasn't seen before, or
// if anything that disappeared.
type Tracker struct {
	stop   chan struct{}
	topics map[string]interface{}

	tags map[string]string

	// poll is the ticker for polling
	poll <-chan time.Time

	// Plugin is the instance plugin to use
	Plugin instance.Plugin

	// Name is the name of the plugin
	Name string
}

func (m *Tracker) getEndpoint() interface{} {
	return "redirect to endpoint (not implemented)"
}

// Init initializes the event plugin and starts working
func (m *Tracker) init() *Tracker {
	m.topics = map[string]interface{}{}
	m.stop = make(chan struct{})

	if m.poll == nil {
		m.poll = time.Tick(3 * time.Second)
	}

	for _, topic := range types.PathsFromStrings(
		"found",
		"lost",
		"error",
	) {
		types.Put(topic, m.getEndpoint, m.topics)
	}
	return m
}

// Stop stops the Tracker
func (m *Tracker) Stop() {
	if m.stop != nil {
		close(m.stop)
	}
}

// List returns the nodes under the given topic
func (m *Tracker) List(topic types.Path) ([]string, error) {
	return types.List(topic, m.topics), nil
}

// PublishOn sets the channel to publish on
func (m *Tracker) PublishOn(events chan<- *event.Event) {
	go func() {

		c := events

		defer func() {
			if c != nil {
				close(c)
			}
		}()

		log.Info("Start trackering instances", "name", m.Name)

		instances := map[instance.ID]instance.Description{}
		last := mapset.NewSet()

	poll:
		for {
			select {
			case <-m.stop:
				m.stop = nil
				return

			case <-m.poll:

				described, err := m.Plugin.DescribeInstances(m.tags, false)
				if err != nil {
					log.Warn("Error describing instances", "name", m.Name, "err", err)
					c <- event.Event{
						Type: eventType,
						ID:   "error",
					}.Init().Now().WithTopic("error").WithDataMust(err)

					// Done here. skip to the next poll
					continue poll
				}

				current := mapset.NewSet()
				for _, f := range described {
					current.Add(f.ID)
					instances[f.ID] = f
				}

				lost := last.Difference(current)
				found := current.Difference(last)

				log.Debug("found", "list", found)
				log.Debug("lost", "list", lost)

				for v := range lost.Iter() {
					id := v.(instance.ID)
					c <- event.Event{
						Type: eventType,
						ID:   string(id),
					}.Init().Now().WithTopic("lost").WithDataMust(instances[id])

					log.Debug("sent lost", "id", id)
				}
				for v := range found.Iter() {
					id := v.(instance.ID)
					c <- event.Event{
						Type: eventType,
						ID:   string(id),
					}.Init().Now().WithTopic("found").WithDataMust(instances[id])

					log.Debug("sent found", "id", id)
				}

				log.Debug("current", "list", current)

				last = current

			}
		}
	}()
}
