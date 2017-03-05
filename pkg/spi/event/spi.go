package event

import (
	"github.com/docker/infrakit/pkg/spi"
)

// InterfaceSpec is the current name and version of the Flavor API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "Event",
	Version: "0.1.0",
}

// Plugin must be implemented for the object to be able to publish events.
type Plugin interface {

	// Topics return a list of topics
	Topics() ([]Topic, error)
}

// Publisher is the interface that event sources also implement to be assigned
// a publish function.
type Publisher interface {

	// PublishOn sets the channel to publish
	PublishOn(chan<- *Event)
}

// Subscriber is the interface given to clients interested in events
type Subscriber interface {

	// SubscribeOn returns the channel for the topic
	SubscribeOn(Topic) (<-chan *Event, error)
}
