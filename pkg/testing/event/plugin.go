package event

import (
	"github.com/docker/infrakit/pkg/spi/event"
)

// Plugin implements event.Plugin.  It is also a publisher. Initialize the struct with (*Publisher)(nil).
type Plugin struct {
	event.Publisher

	// DoTopics implements Topics via function
	DoTopics func() (child []event.Topic, err error)
}

// Topics lists the topics
func (t *Plugin) Topics() (child []event.Topic, err error) {
	return t.DoTopics()
}

// Publisher implements the event.Publisher interface
type Publisher struct {

	// DoPublishOn sets the channel to publish on.
	DoPublishOn func(chan<- *event.Event)
}

// PublishOn sets the publish channel
func (t *Publisher) PublishOn(c chan<- *event.Event) {
	if t == nil {
		return
	}
	t.DoPublishOn(c)
}
