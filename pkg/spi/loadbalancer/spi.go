package loadbalancer

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// InterfaceSpec is the current name and version of the L4 API.
var InterfaceSpec = spi.InterfaceSpec{
	Name:    "L4",
	Version: "0.6.1",
}

// Route is a description of a network target for a load balancer.
type Route struct {
	// Port is the TCP port that the backend instance is listening on.
	Port int

	// Protocol is the network protocol that the backend instance is listening on.
	Protocol Protocol

	// LoadBalancerPort is the TCP port that the load balancer is listening on.
	LoadBalancerPort int

	// LoadBalancerProtocol is the network protocol that the load balancer is listening on.
	LoadBalancerProtocol Protocol

	// Certificate is the certificate used by the load balancer.
	Certificate *string

	// HealthMonitorPath is the url path used by the route health monitor
	HealthMonitorPath *string
}

// Validate validates the data herein. If necessary, some data values will be mutated as needed.
func (r *Route) Validate() error {
	if r.Port == 0 {
		return fmt.Errorf("no port")
	}
	if r.LoadBalancerPort == 0 {
		return fmt.Errorf("no loadbalancer port")
	}
	if !r.Protocol.Valid() {
		return fmt.Errorf("bad protocol: %v", r.Protocol)
	}
	if !r.LoadBalancerProtocol.Valid() {
		return fmt.Errorf("bad loadbalancer protocol: %v", r.LoadBalancerProtocol)
	}
	if r.LoadBalancerProtocol == HTTPS && r.Certificate == nil {
		return fmt.Errorf("HTTPS but no certificate")
	}

	return nil
}

// HealthCheck models the a probe that checks against a given backend port at given
// intervals and with timeout.
type HealthCheck struct {
	// BackendPort is the port on a backend node to probe
	BackendPort int
	// Healthy is the number of tries to succeed to be considered healthy
	Healthy int
	// Unhealthy is the number of tries to fail to be considered unhealthy
	Unhealthy int
	// Interval is the probe / ping interval
	Interval time.Duration
	// Timeout is the duration to wait before timing out.
	Timeout time.Duration
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

	// Unpublish dissociates the load balancer from the backend service at the given port.
	Unpublish(extPort int) (Result, error)

	// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
	// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
	// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
	// at the interval specified.
	ConfigureHealthCheck(hc HealthCheck) (Result, error)

	// RegisterBackend registers instances identified by the IDs to the LB's backend pool
	RegisterBackends(ids []instance.ID) (Result, error)

	// DeregisterBackend removes the specified instances from the backend pool
	DeregisterBackends(id []instance.ID) (Result, error)

	// Backends returns a list of backends
	Backends() ([]instance.ID, error)
}
