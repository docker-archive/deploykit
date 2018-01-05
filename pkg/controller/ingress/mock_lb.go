package ingress

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// Result string
type lbResult string

func (r lbResult) String() string {
	return string(r)
}

type mocklb struct {
	name         string
	routes       []loadbalancer.Route
	publishErr   error
	unpublishErr error
}

// NewMockLBPlugin returns a mock L4 loadbalancer
func NewMockLBPlugin(mockRoutes []loadbalancer.Route) loadbalancer.L4 {
	lb := &mocklb{
		name:   "mocklb",
		routes: mockRoutes,
	}

	return lb
}

// Name is the name of the load balancer
func (l *mocklb) Name() string {
	return l.name
}

// Routes lists all known routes.
func (l *mocklb) Routes() ([]loadbalancer.Route, error) {
	routes := make([]loadbalancer.Route, len(l.routes))
	copy(routes, l.routes)
	return routes, nil
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l *mocklb) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	if l.publishErr != nil {
		return nil, l.publishErr
	}

	l.routes = append(l.routes, route)

	return lbResult("publish"), nil
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l *mocklb) Unpublish(extPort int) (loadbalancer.Result, error) {
	if l.unpublishErr != nil {
		return nil, l.unpublishErr
	}

	for index, route := range l.routes {
		if route.LoadBalancerPort == extPort {
			// Remove the route
			l.routes = append((l.routes)[:index], (l.routes)[index+1:]...)
		}
	}

	return lbResult("unpublish"), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (l *mocklb) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	return lbResult("healthcheck"), nil
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool.
func (l *mocklb) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return lbResult("register"), nil
}

// DeregisterBackend removes the specified instances from the backend pool.
func (l *mocklb) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return lbResult("deregister"), nil
}

// Backends returns a list of backends
func (l *mocklb) Backends() ([]instance.ID, error) {
	return []instance.ID{}, nil
}
