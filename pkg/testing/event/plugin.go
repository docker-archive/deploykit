package event

import (
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements event.Plugin.  It is also a publisher. Initialize the struct with (*Publisher)(nil).
type Plugin struct {
	event.Publisher

	// DoList implements List via function
	DoList func(topic types.Path) (child []string, err error)
}

// List lists the topics
func (t *Plugin) List(topic types.Path) (child []string, err error) {
	return t.DoList(topic)
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
