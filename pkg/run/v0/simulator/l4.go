package simulator

import (
	"fmt"
	"sync"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/store/file"
	"github.com/docker/infrakit/pkg/store/mem"
	"github.com/docker/infrakit/pkg/types"
)

var l4Logger = logutil.New("module", "simulator/l4")

// NewL4 returns a L4 loadbalancer
func NewL4(name string, options Options) loadbalancer.L4 {
	l := &l4Simulator{
		name: name,
	}

	switch options.Store {
	case StoreFile:
		l.routes = file.NewStore("route", options.Dir)
		l.backends = file.NewStore("backend", options.Dir)
		l.healthchecks = file.NewStore("healthcheck", options.Dir)
	case StoreMem:
		l.routes = mem.NewStore("route")
		l.backends = mem.NewStore("backend")
		l.healthchecks = mem.NewStore("healthcheck")
	}
	return l
}

// This is an example L4 simulator that shows how to implement a L4 plugin that can
// be controlled by the ingress controller.
type l4Simulator struct {
	name         string
	routes       store.KV
	backends     store.KV
	healthchecks store.KV
	lock         sync.Mutex
}

// Name is the name of the load balancer
func (l *l4Simulator) Name() string {
	l4Logger.Debug("Name", "V", debugV)
	return l.name
}

type result string

func (r result) String() string {
	return string(r)
}

// Routes lists all known routes.
func (l *l4Simulator) Routes() ([]loadbalancer.Route, error) {
	l4Logger.Debug("Routes", "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	out := []loadbalancer.Route{}
	err := store.Visit(l.routes, nil, nil,
		func(buff []byte) (interface{}, error) {
			route := loadbalancer.Route{}
			err := decode(buff, &route)
			return route, err
		},
		func(o interface{}) (bool, error) {
			out = append(out, o.(loadbalancer.Route))
			return true, nil
		})
	return out, err
}

func mustEncode(v interface{}) []byte {
	buff, err := types.AnyValueMust(v).MarshalYAML()
	if err != nil {
		panic(err)
	}
	return buff
}

func decode(buff []byte, v interface{}) error {
	a, err := types.AnyYAML(buff)
	if err != nil {
		return err
	}
	return a.Decode(v)
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l *l4Simulator) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	l4Logger.Debug("Public", "name", l.name, "route", route, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	exists, err := l.routes.Exists(route.LoadBalancerPort)
	if err != nil {
		return result(""), err
	}
	if exists {
		return result(""), fmt.Errorf("duplicate port %v", route.LoadBalancerPort)
	}

	return result("publish"), l.routes.Write(route.LoadBalancerPort, mustEncode(route))
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l *l4Simulator) Unpublish(extPort int) (loadbalancer.Result, error) {
	l4Logger.Debug("Unpublish", "name", l.name, "extPort", extPort, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	exists, err := l.routes.Exists(extPort)
	if err != nil {
		return result(""), err
	}
	if !exists {
		return result(""), fmt.Errorf("unknown port %v", extPort)
	}
	return result("unpublish"), l.routes.Delete(extPort)
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
// at the interval specified.
func (l *l4Simulator) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	l4Logger.Debug("ConfigureHealthCheck", "name", l.name, "heathCheck", hc, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	return result("healthcheck"), l.routes.Write(hc.BackendPort, mustEncode(hc))
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (l *l4Simulator) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	l4Logger.Debug("RegisterBackends", "name", l.name, "ids", ids, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, id := range ids {
		err := l.backends.Write(id, mustEncode(id))
		if err != nil {
			return result("err"), err
		}
	}
	return result("ok"), nil
}

// DeregisterBackend removes the specified instances from the backend pool
func (l *l4Simulator) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	l4Logger.Debug("DeregisterBackends", "name", l.name, "ids", ids, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, id := range ids {
		l.backends.Delete(id)
	}
	return result("ok"), nil
}

// Backends returns a list of backends
func (l *l4Simulator) Backends() ([]instance.ID, error) {
	l4Logger.Debug("Backends", "name", l.name, "V", debugV)
	l.lock.Lock()
	defer l.lock.Unlock()

	out := []instance.ID{}

	defer l4Logger.Debug("Backends", "name", l.name, "V", debugV, "backends", out)

	err := store.Visit(l.backends, nil, nil,
		func(buff []byte) (interface{}, error) {
			var backend instance.ID
			err := decode(buff, &backend)
			return backend, err
		},
		func(o interface{}) (bool, error) {
			out = append(out, o.(instance.ID))
			return true, nil
		})
	return out, err
}
