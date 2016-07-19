package loadbalancer

import (
	"fmt"
	"time"
)

// State is the state of the load balancer with a string representation.
type State interface {
	fmt.Stringer

	// GetName returns the name of this load balancer
	GetName() string

	// HashListener returns the backendPort and true if the listener exists.
	HasListener(extPort uint32, protocol Protocol) (uint32, bool)

	// VisitListeners provides a mechanism for caller to iterate through all the listeners
	VisitListeners(v func(lbPort, instancePort uint32, protocol Protocol))
}

// Result is the result of an operation
type Result interface {
	fmt.Stringer
}

// L4Provisioner is the generic provisioner for a signle L4 load balancer instance
type L4Provisioner interface {

	// Name is the name of the load balancer
	Name() string

	// State returns the current state of the load balancer
	State() (State, error)

	// PublishService publishes a service in the LB by adding a load balancing rule
	PublishService(ext Protocol, extPort uint32, backend Protocol, backendPort uint32) (Result, error)

	// UnpublishService dissociates the load balancer from the backend service at the given port.
	UnpublishService(extPort uint32) (Result, error)

	// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
	ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int, interval, timeout time.Duration) (Result, error)

	// RegisterBackend registers instances identified by the IDs to the LB's backend pool
	RegisterBackend(id string, more ...string) (Result, error)

	// DeregisterBackend removes the specified instances from the backend pool
	DeregisterBackend(id string, more ...string) (Result, error)
}
