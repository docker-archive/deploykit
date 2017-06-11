package instance

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	monitorType = event.Type("instance-monitor")
)

// Monitor implements the event spi -- just just calls the Describe to get a
// list of known instances, and report anything it hasn't seen before, or
// if anything that disappeared.
type Monitor struct {
	stop   chan struct{}
	topics map[string]interface{}

	// Plugin is the instance plugin to use
	Plugin instance.Plugin
}

func (m *Monitor) getEndpoint() interface{} {
	return "redirect to endpoint (not implemented)"
}

// Init initializes the event plugin and starts working
func (m *Monitor) Init() event.Plugin {
	m.topics = map[string]interface{}{}
	m.stop = make(chan struct{})

	for _, topic := range types.PathFromStrings(
		"found",
		"lost",
		"error",
	) {
		types.Put(topic, m.getEndpoint, m.topics)
	}
	return m
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
}

// List returns the nodes under the given topic
func (m *Monitor) List(topic types.Path) ([]string, error) {
	return types.List(topic, m.topics), nil
}

// PublishOn sets the channel to publish on
func (m *Monitor) PublishOn(c chan<- *event.Event) {
	go func() {
		defer close(c)

		log.Infoln("Start monitoring instances", c)

		ticker := time.Tick(2 * time.Second)

		instances := map[instance.ID]instance.Description{}
		last := mapset.NewSet()

		for {
			select {
			case <-m.stop:
				return

			case <-ticker:

				described, err := m.Plugin.DescribeInstances(nil, true)
				if err != nil {
					log.Warningln("****** err", err)
					c <- event.Event{
						Type: monitorType,
						ID:   "error",
					}.Init().Now().WithTopic("error").WithDataMust(err)
				}

				current := mapset.NewSet()
				for _, f := range described {
					current.Add(f.ID)
					instances[f.ID] = f
				}

				lost := last.Difference(current)
				found := current.Difference(last)

				log.Debugln("***** found=", found)
				log.Debugln("***** lost=", lost)

				for v := range lost.Iter() {
					id := v.(instance.ID)
					c <- event.Event{
						Type: monitorType,
						ID:   string(id),
					}.Init().Now().WithTopic("lost").WithDataMust(instances[id])
				}
				for v := range found.Iter() {
					id := v.(instance.ID)
					c <- event.Event{
						Type: monitorType,
						ID:   string(id),
					}.Init().Now().WithTopic("found").WithDataMust(instances[id])
				}

				log.Debugln("***** current=", current)

				last = current

			}
		}
	}()
}
