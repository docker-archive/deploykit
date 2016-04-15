package libmachete

// EventType is the identifier for a creation event.
type EventType int

const (
	// CreateStarted indicates that creation has begun.
	CreateStarted EventType = iota

	// CreateCompleted indicates that creation was successful.
	CreateCompleted

	// CreateError indicates a problem creating the resource.
	CreateError
)

// A CreateEvent signals a state change in the creation process.
type CreateEvent struct {
	Type       EventType
	Error      error
	ResourceID string
}

// A Provisioner is a vendor-agnostic API used to create and manage
// resources with an infrastructure provider.
type Provisioner interface {
	Create(request interface{}) (<-chan CreateEvent, error)
}
