package loadbalancer

import (
	"fmt"
	"time"
)

// Route is a description of a network target for a load balancer.
type Route struct {
	// Port is the TCP port that the backend instance is listening on.
	Port uint32

	// Protocol is the network protocol that the load balancer is routing.
	Protocol Protocol

	// LoadBalancerPort is the TCP port that the load balancer is listening on.
	LoadBalancerPort uint32

	Certificate *string
}

// Result is the result of an operation
type Result interface {
	fmt.Stringer
}

// TODO(chungers) -- Update the interface to support Vhosts for L7 routing.

// L4 is the generic driver for a single L4 load balancer instance
type L4 interface {

	// Name is the name of the load balancer
	Name() string

	// Routes lists all known routes.
	Routes() ([]Route, error)

	// Publish publishes a route in the LB by adding a load balancing rule
	Publish(route Route) (Result, error)

	// UnpublishService dissociates the load balancer from the backend service at the given port.
	Unpublish(extPort uint32) (Result, error)

	// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
	// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
	// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
	// at the interval specified.
	ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int, interval, timeout time.Duration) (Result, error)

	// RegisterBackend registers instances identified by the IDs to the LB's backend pool
	RegisterBackend(id string, more ...string) (Result, error)

	// DeregisterBackend removes the specified instances from the backend pool
	DeregisterBackend(id string, more ...string) (Result, error)
}
