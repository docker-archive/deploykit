package loadbalancer

import (
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// PluginServerWithNames returns a loadbalancer map of plugins by name
func PluginServerWithNames(list func() (map[string]loadbalancer.L4, error)) *L4 {
	keyed := internal.ServeKeyed(

		// This is where templates would be nice...
		func() (map[string]interface{}, error) {
			m, err := list()
			if err != nil {
				return nil, err
			}
			out := map[string]interface{}{}
			for k, v := range m {
				out[k] = v
			}
			return out, nil
		},
	)

	return &L4{
		keyed: keyed,
	}
}

// PluginServer returns a L4 load balancer that conforms to the net/rpc rpc call convention.
func PluginServer(l4 loadbalancer.L4) *L4 {
	return &L4{keyed: internal.ServeSingle(l4)}
}

// L4 is the exported type for json-rpc
type L4 struct {
	keyed *internal.Keyed
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See plugin.Vendor
func (l4 *L4) VendorInfo() *spi.VendorInfo {
	base, _ := l4.keyed.Keyed(plugin.Name("."))
	if m, is := base.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (l4 *L4) ImplementedInterface() spi.InterfaceSpec {
	return loadbalancer.InterfaceSpec
}

// Objects returns the objects exposed by this kind of RPC service
func (l4 *L4) Objects() []rpc.Object {
	return l4.keyed.Objects()
}

// Name returns the name of the load balancer
func (l4 *L4) Name(_ *http.Request, req *NameRequest, resp *NameResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		name := v.(loadbalancer.L4).Name()
		resp.Name = name
		return nil
	})
}

// Routes lists all known routes.
func (l4 *L4) Routes(_ *http.Request, req *RoutesRequest, resp *RoutesResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		routes, err := v.(loadbalancer.L4).Routes()
		if err == nil {
			resp.Routes = routes
		}
		return err
	})
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l4 *L4) Publish(_ *http.Request, req *PublishRequest, resp *PublishResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).Publish(req.Route)
		if err == nil {
			resp.Result = result.String()
		}
		return err
	})
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l4 *L4) Unpublish(_ *http.Request, req *UnpublishRequest, resp *UnpublishResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).Unpublish(req.ExtPort)
		if err == nil {
			resp.Result = result.String()
		}
		return err
	})
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
func (l4 *L4) ConfigureHealthCheck(_ *http.Request, req *ConfigureHealthCheckRequest, resp *ConfigureHealthCheckResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).ConfigureHealthCheck(req.HealthCheck)
		if err == nil {
			resp.Result = result.String()
		}
		return err
	})
}

// RegisterBackends registers instances identified by the IDs to the LB's backend pool
func (l4 *L4) RegisterBackends(_ *http.Request, req *RegisterBackendsRequest, resp *RegisterBackendsResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).RegisterBackends(req.IDs)
		if err == nil {
			resp.Result = result.String()
		}
		return err
	})
}

// DeregisterBackends removes the specified instances from the backend pool
func (l4 *L4) DeregisterBackends(_ *http.Request, req *DeregisterBackendsRequest, resp *DeregisterBackendsResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).DeregisterBackends(req.IDs)
		if err == nil {
			resp.Result = result.String()
		}
		return err
	})
}

// Backends returns the list of backends
func (l4 *L4) Backends(_ *http.Request, req *BackendsRequest, resp *BackendsResponse) error {
	return l4.keyed.Do(req, func(v interface{}) error {
		result, err := v.(loadbalancer.L4).Backends()
		if err == nil {
			resp.IDs = result
		}
		return err
	})
}
