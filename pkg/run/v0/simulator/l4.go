package simulator

import (
	"fmt"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var l4Logger = logutil.New("module", "simulator/l4")

type l4Simulator struct {
	name     string
	routes   map[uint32]loadbalancer.Route
	backends map[instance.ID]instance.ID
	lock     sync.Mutex
}

func (l *l4Simulator) alloc() *l4Simulator {
	l.routes = map[uint32]loadbalancer.Route{}
	l.backends = map[instance.ID]instance.ID{}

	return l
}

// Name is the name of the load balancer
func (l *l4Simulator) Name() string {
	l4Logger.Info("Name")
	return l.name
}

// Routes lists all known routes.
func (l *l4Simulator) Routes() ([]loadbalancer.Route, error) {
	l4Logger.Info("Routes")
	l.lock.Lock()
	defer l.lock.Unlock()

	out := []loadbalancer.Route{}
	for _, v := range l.routes {
		out = append(out, v)
	}
	return out, nil
}

type result string

func (r result) String() string {
	return string(r)
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l *l4Simulator) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	l4Logger.Info("Public", "name", l.name, "route", route)
	l.lock.Lock()
	defer l.lock.Unlock()
	_, has := l.routes[route.LoadBalancerPort]
	if has {
		return result(""), fmt.Errorf("duplicate port %v", route.LoadBalancerPort)
	}
	l.routes[route.LoadBalancerPort] = route
	return result("ok"), nil
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l *l4Simulator) Unpublish(extPort uint32) (loadbalancer.Result, error) {
	l4Logger.Info("Unpublish", "name", l.name, "extPort", extPort)
	l.lock.Lock()
	defer l.lock.Unlock()
	_, has := l.routes[extPort]
	if !has {
		return result(""), fmt.Errorf("unknown port %v", extPort)
	}
	delete(l.routes, extPort)
	return result("ok"), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
// at the interval specified.
func (l *l4Simulator) ConfigureHealthCheck(backendPort uint32, healthy,
	unhealthy int, interval, timeout time.Duration) (loadbalancer.Result, error) {
	l4Logger.Info("ConfigureHealthCheck",
		"name", l.name,
		"backendPort", backendPort,
		"healthy", healthy,
		"unhealthy", unhealthy,
		"interval", interval,
		"timeout", timeout)

	return result("ok"), nil
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (l *l4Simulator) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	l4Logger.Info("RegisterBackends", "name", l.name, "ids", ids)
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, id := range ids {
		_, has := l.backends[id]
		if !has {
			l.backends[id] = id
		}
	}
	return result("ok"), nil
}

// DeregisterBackend removes the specified instances from the backend pool
func (l *l4Simulator) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	l4Logger.Info("DeregisterBackends", "name", l.name, "ids", ids)
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, id := range ids {
		_, has := l.backends[id]
		if has {
			delete(l.backends, id)
		}
	}
	return result("ok"), nil
}

// Backends returns a list of backends
func (l *l4Simulator) Backends() ([]instance.ID, error) {
	l4Logger.Info("Backends", "name", l.name)
	l.lock.Lock()
	defer l.lock.Unlock()

	out := []instance.ID{}
	for k := range l.backends {
		out = append(out, k)
	}
	return out, nil
}
