package loadbalancer

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// NameRequest is the rpc wrapper for the Name request
type NameRequest struct {
	Type string
}

// NameResponse is the rpc wrapper for the Name response
type NameResponse struct {
	Type string
	Name string
}

// RoutesRequest is the rpc wrapper for Routes request
type RoutesRequest struct {
	Type string
}

// RoutesResponse is the rpc wrapper for Routes response
type RoutesResponse struct {
	Type   string
	Routes []loadbalancer.Route
}

// PublishRequest is the rpc wrapper for Publish request
type PublishRequest struct {
	Type  string
	Route loadbalancer.Route
}

// PublishResponse is the rpc wrapper for Publish response
type PublishResponse struct {
	Type   string
	Result string
}

// UnpublishRequest is the rpc wrapper for Unpublish request
type UnpublishRequest struct {
	Type    string
	ExtPort int
}

// UnpublishResponse is the rpc wrapper for Unpublish response
type UnpublishResponse struct {
	Type   string
	Result string
}

// ConfigureHealthCheckRequest is the rpc wrapper for ConfigureHealthCheck request
type ConfigureHealthCheckRequest struct {
	Type                     string
	loadbalancer.HealthCheck `json:",inline" yaml:",inline"`
}

// ConfigureHealthCheckResponse is the rpc wrapper for ConfigureHealthCheck response
type ConfigureHealthCheckResponse struct {
	Type   string
	Result string
}

// RegisterBackendsRequest is the rpc wrapper for RegisterBackend request
type RegisterBackendsRequest struct {
	Type string
	IDs  []instance.ID
}

// RegisterBackendsResponse is the rpc wrapper for RegisterBackend response
type RegisterBackendsResponse struct {
	Type   string
	Result string
}

// DeregisterBackendsRequest is the rpc wrapper for DeregisterBackend request
type DeregisterBackendsRequest struct {
	Type string
	IDs  []instance.ID
}

// DeregisterBackendsResponse is the rpc wrapper for DeregisterBackend response
type DeregisterBackendsResponse struct {
	Type   string
	Result string
}

// BackendsRequest is the rpc wrapper for Backends request
type BackendsRequest struct {
	Type string
}

// BackendsResponse is the rpc response for Backends call
type BackendsResponse struct {
	Type string
	IDs  []instance.ID
}
