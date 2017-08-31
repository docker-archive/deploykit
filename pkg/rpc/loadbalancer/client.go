package loadbalancer

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// clientResult implements loadbalancer.Result
type clientResult string

func (f clientResult) String() string {
	return string(f)
}

// NewClient returns a plugin interface implementation connected to a plugin
func NewClient(name plugin.Name, socketPath string) (loadbalancer.L4, error) {
	rpcClient, err := rpc_client.New(socketPath, loadbalancer.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{name: name, client: rpcClient}, nil
}

// Adapt converts a rpc client to a Plugin object
func Adapt(name plugin.Name, rpcClient rpc_client.Client) loadbalancer.L4 {
	return &client{name: name, client: rpcClient}
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

// Name is the name of the load balancer.
func (c client) Name() string {
	_, l4Type := c.name.GetLookupAndType()
	req := NameRequest{Type: l4Type}
	resp := NameResponse{}

	c.client.Call("L4.Name", req, &resp)
	return resp.Name
}

// Routes lists all known routes.
func (c client) Routes() ([]loadbalancer.Route, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := RoutesRequest{Type: l4Type}
	resp := RoutesResponse{}

	if err := c.client.Call("L4.Routes", req, &resp); err != nil {
		return nil, err
	}
	return resp.Routes, nil
}

// Publish publishes a route in the LB by adding a load balancing rule
func (c client) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := PublishRequest{Type: l4Type, Route: route}
	resp := PublishResponse{}

	if err := c.client.Call("L4.Publish", req, &resp); err != nil {
		return nil, err
	}
	return clientResult(resp.Result), nil
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (c client) Unpublish(extPort int) (loadbalancer.Result, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := UnpublishRequest{Type: l4Type, ExtPort: extPort}
	resp := UnpublishResponse{}

	if err := c.client.Call("L4.Unpublish", req, &resp); err != nil {
		return nil, err
	}
	return clientResult(resp.Result), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (c client) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := ConfigureHealthCheckRequest{
		Type:        l4Type,
		HealthCheck: hc,
	}
	resp := ConfigureHealthCheckResponse{}

	if err := c.client.Call("L4.ConfigureHealthCheck", req, &resp); err != nil {
		return nil, err
	}
	return clientResult(resp.Result), nil
}

// RegisterBackends registers instances identified by the IDs to the LB's backend pool
func (c client) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := RegisterBackendsRequest{
		Type: l4Type,
		IDs:  ids,
	}
	resp := RegisterBackendsResponse{}

	if err := c.client.Call("L4.RegisterBackends", req, &resp); err != nil {
		return nil, err
	}
	return clientResult(resp.Result), nil
}

// DeregisterBackends removes the specified instances from the backend pool
func (c client) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := DeregisterBackendsRequest{
		Type: l4Type,
		IDs:  ids,
	}
	resp := DeregisterBackendsResponse{}

	if err := c.client.Call("L4.DeregisterBackends", req, &resp); err != nil {
		return nil, err
	}
	return clientResult(resp.Result), nil
}

// Backends returns the backends as list of instance ids
func (c client) Backends() ([]instance.ID, error) {
	_, l4Type := c.name.GetLookupAndType()
	req := BackendsRequest{
		Type: l4Type,
	}
	resp := BackendsResponse{}
	if err := c.client.Call("L4.Backends", req, &resp); err != nil {
		return nil, err
	}
	return resp.IDs, nil
}
