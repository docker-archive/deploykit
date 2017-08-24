package loadbalancer

import (
	"github.com/docker/infrakit/pkg/spi/instance"
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
	DoUnpublish func(extPort int) (loadbalancer.Result, error)

	// DoConfigureHealthCheck configures the health checks for instance removal and reconfiguration
	DoConfigureHealthCheck func(hc loadbalancer.HealthCheck) (loadbalancer.Result, error)

	// RegisterBackends registers instances identified by the IDs to the LB's backend pool
	DoRegisterBackends func(ids []instance.ID) (loadbalancer.Result, error)

	// DoDeregisterBackend removes the specified instances from the backend pool
	DoDeregisterBackends func(ids []instance.ID) (loadbalancer.Result, error)

	// DoBackends returns the list of instance ids in the backend
	DoBackends func() ([]instance.ID, error)
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
func (l4 *L4) Unpublish(extPort int) (loadbalancer.Result, error) {
	return l4.DoUnpublish(extPort)
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (l4 *L4) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	return l4.DoConfigureHealthCheck(hc)
}

// RegisterBackends registers instances identified by the IDs to the LB's backend pool
func (l4 *L4) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return l4.DoRegisterBackends(ids)
}

// DeregisterBackends removes the specified instances from the backend pool
func (l4 *L4) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return l4.DoDeregisterBackends(ids)
}

// Backends returns the list of instance ids as the backend
func (l4 *L4) Backends() ([]instance.ID, error) {
	return l4.DoBackends()
}
