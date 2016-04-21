package api

// CreateInstanceEventType is the identifier for an instance create event.
type CreateInstanceEventType int

// DestroyInstanceEventType is the identifier for an instance destroy event.
type DestroyInstanceEventType int

const (
	// CreateInstanceStarted indicates that creation has begun.
	CreateInstanceStarted CreateInstanceEventType = iota

	// CreateInstanceCompleted indicates that creation was successful.
	CreateInstanceCompleted

	// CreateInstanceError indicates a problem creating the instance.
	CreateInstanceError

	// DestroyInstanceStarted indicates that destruction has begun.
	DestroyInstanceStarted DestroyInstanceEventType = iota

	// DestroyInstanceCompleted indicates that destruction was successful.
	DestroyInstanceCompleted

	// DestroyInstanceError indicates a problem destroying the instance.
	DestroyInstanceError
)

// A CreateInstanceEvent signals a state change in the instance create process.
type CreateInstanceEvent struct {
	Type       CreateInstanceEventType
	Error      error
	InstanceID string
}

// A DestroyInstanceEvent signals a state change in the instance destroy process.
type DestroyInstanceEvent struct {
	Type  DestroyInstanceEventType
	Error error
}

// MachineRequest defines the basic attributes that any provisioner's creation request must define.
type MachineRequest interface {
	GetName() string
}

// A Provisioner is a vendor-agnostic API used to create and manage
// resources with an infrastructure provider.
type Provisioner interface {
	// NewRequestInstance retrieves a new instance of the request type consumed by
	// CreateInstance.
	NewRequestInstance() MachineRequest

	CreateInstance(request MachineRequest) (<-chan CreateInstanceEvent, error)

	DestroyInstance(instanceID string) (<-chan DestroyInstanceEvent, error)
}
