package loadbalancer

import (
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// PluginServer returns a L4 load balancer that conforms to the net/rpc rpc call convention.
func PluginServer(l4 loadbalancer.L4) *L4 {
	return &L4{l4: l4}
}

// L4 is the exported type for json-rpc
type L4 struct {
	l4 loadbalancer.L4
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (l4 *L4) ImplementedInterface() spi.InterfaceSpec {
	return loadbalancer.InterfaceSpec
}

// Types returns the types exposed by this kind of RPC service
func (l4 *L4) Types() []string {
	return []string{"."} // no types
}

// Name returns the name of the load balancer
func (l4 *L4) Name(_ *http.Request, req *NameRequest, resp *NameResponse) error {
	name := l4.l4.Name()
	resp.Name = name
	return nil
}

// Routes lists all known routes.
func (l4 *L4) Routes(_ *http.Request, req *RoutesRequest, resp *RoutesResponse) error {
	routes, err := l4.l4.Routes()
	if err == nil {
		resp.Routes = routes
	}
	return err
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l4 *L4) Publish(_ *http.Request, req *PublishRequest, resp *PublishResponse) error {
	result, err := l4.l4.Publish(req.Route)
	if err == nil {
		resp.Result = result.String()
	}
	return err
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l4 *L4) Unpublish(_ *http.Request, req *UnpublishRequest, resp *UnpublishResponse) error {
	result, err := l4.l4.Unpublish(req.ExtPort)
	if err == nil {
		resp.Result = result.String()
	}
	return err
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (l4 *L4) ConfigureHealthCheck(_ *http.Request, req *ConfigureHealthCheckRequest, resp *ConfigureHealthCheckResponse) error {
	result, err := l4.l4.ConfigureHealthCheck(
		req.BackendPort,
		req.Healthy,
		req.Unhealthy,
		req.Interval,
		req.Timeout,
	)
	if err == nil {
		resp.Result = result.String()
	}
	return err
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (l4 *L4) RegisterBackends(_ *http.Request, req *RegisterBackendsRequest, resp *RegisterBackendsResponse) error {
	result, err := l4.l4.RegisterBackends(req.IDs)
	if err == nil {
		resp.Result = result.String()
	}
	return err
}

// DeregisterBackend removes the specified instances from the backend pool
func (l4 *L4) DeregisterBackends(_ *http.Request, req *DeregisterBackendsRequest, resp *DeregisterBackendsResponse) error {
	result, err := l4.l4.DeregisterBackends(req.IDs)
	if err == nil {
		resp.Result = result.String()
	}
	return err
}

// Backends returns the list of backends
func (l4 *L4) Backends(_ *http.Request, req *BackendsRequest, resp *BackendsResponse) error {
	result, err := l4.l4.Backends()
	if err == nil {
		resp.IDs = result
	}
	return err
}
