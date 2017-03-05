package event

import (
	"path"
	"sort"
	"time"

	"github.com/docker/infrakit/pkg/types"
)

// Type is the type of an event.  This gives hint about what struct types to map to, etc.
// It also marks one instance of an event as of different nature from another.
type Type string

const (
	// TypeError is the type to use for sending errors in the transport of the events.
	TypeError = Type("error")
)

// Topic is the topic
type Topic string

type topicSorter []Topic

func (p topicSorter) Len() int           { return len(p) }
func (p topicSorter) Less(i, j int) bool { return string(p[i]) < string(p[j]) }
func (p topicSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort sorts the topics
func Sort(p []Topic) {
	sort.Sort(topicSorter(p))
}

// NewTopic returns a topic
func NewTopic(s string) Topic {
	return Topic(s)
}

// Under return the topic under the parent
func (child Topic) Under(parent Topic) Topic {
	return Topic(path.Join(string(parent), string(child)))
}

// String return the string representation
func (self Topic) String() string {
	return string(self)
}

// Event holds information about when, what, etc.
type Event struct {

	// Topic is the topic to which this event is published
	Topic Topic

	// Type of the event. This is usually used as a hint to what struct types to use to unmarshal data
	Type Type `json:",omitempty"`

	// ID is unique id for the event.
	ID string

	// Timestamp is the time.UnixNano() value -- this is the timestamp when event occurred
	Timestamp time.Time

	// Received is the timestamp when the message is received.
	Received time.Time `json:",omitempty"`

	// Data contains some application specific payload
	Data *types.Any `json:",omitempty"`

	// Error contains any errors that occurred during delivery of the mesasge
	Error error `json:",omitempty"`
}

// Init creates an instance with the value initialized to the state of receiver.
func (event Event) Init() *Event {
	copy := event
	return &copy
}

// WithError sets the error
func (event *Event) WithError(err error) *Event {
	event.Error = err
	return event
}

// WithTopic sets the topic from input string
func (event *Event) WithTopic(s string) *Event {
	event.Topic = Topic(s)
	return event
}

// WithType sets the type from a string
func (event *Event) WithType(s string) *Event {
	event.Type = Type(s)
	return event
}

// Now sets the timestamp of this event to now.
func (event *Event) Now() *Event {
	event.Timestamp = time.Now()
	return event
}

// ReceivedNow marks the receipt timestamp with now
func (event *Event) ReceivedNow() *Event {
	event.Received = time.Now()
	return event
}

// WithData converts the data into an any and sets this event's data to it.
func (event *Event) WithData(data interface{}) (*Event, error) {
	any, err := types.AnyValue(data)
	if err != nil {
		return nil, err
	}
	event.Data = any
	return event, nil
}

// WithDataMust does what WithData does but will panic on error
func (event *Event) WithDataMust(data interface{}) *Event {
	e, err := event.WithData(data)
	if err != nil {
		panic(err)
	}
	return e
}

// FromAny sets the fields of this Event from the contents of Any
func (event *Event) FromAny(any *types.Any) *Event {
	err := any.Decode(event)
	if err != nil {
		event.Error = err
	}
	return event
}
