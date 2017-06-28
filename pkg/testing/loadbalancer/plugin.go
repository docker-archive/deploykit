package loadbalancer

import (
	"time"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// L4 implements the loadbalancer.L4 interface and supports testing by letting user assemble behavior dyanmically.
type L4 struct {
	// DoName is the name of the load balancer
	DoName func() string

	// DoRoutes lists all known routes.
	DoRoutes func() ([]loadbalancer.Route, error)

	// DoPublish publishes a route in the LB by adding a load balancing rule
	DoPublish func(route loadbalancer.Route) (loadbalancer.Result, error)

	// DoUnpublish dissociates the load balancer from the backend service at the given port.
	DoUnpublish func(extPort uint32) (loadbalancer.Result, error)

	// DoConfigureHealthCheck configures the health checks for instance removal and reconfiguration
	DoConfigureHealthCheck func(backendPort uint32, healthy, unhealthy int, interval, timeout time.Duration) (loadbalancer.Result, error)

	// RegisterBackend registers instances identified by the IDs to the LB's backend pool
	DoRegisterBackend func(id string, more ...string) (loadbalancer.Result, error)

	// DoDeregisterBackend removes the specified instances from the backend pool
	DoDeregisterBackend func(id string, more ...string) (loadbalancer.Result, error)
}

// Name is the name of the load balancer
func (l4 *L4) Name() string {
	return l4.DoName()
}

// Routes lists all known routes.
func (l4 *L4) Routes() ([]loadbalancer.Route, error) {
	return l4.DoRoutes()
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l4 *L4) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	return l4.DoPublish(route)
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l4 *L4) Unpublish(extPort uint32) (loadbalancer.Result, error) {
	return l4.DoUnpublish(extPort)
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (l4 *L4) ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int, interval, timeout time.Duration) (loadbalancer.Result, error) {
	return l4.DoConfigureHealthCheck(backendPort, healthy, unhealthy, interval, timeout)
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (l4 *L4) RegisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	return l4.DoRegisterBackend(id, more...)
}

// DeregisterBackend removes the specified instances from the backend pool
func (l4 *L4) DeregisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	return l4.DoDeregisterBackend(id, more...)
}
