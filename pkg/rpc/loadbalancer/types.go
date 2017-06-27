package loadbalancer

import (
	"time"

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
	ExtPort uint32
}

// UnpublishResponse is the rpc wrapper for Unpublish response
type UnpublishResponse struct {
	Type   string
	Result string
}

// ConfigureHealthCheckRequest is the rpc wrapper for ConfigureHealthCheck request
type ConfigureHealthCheckRequest struct {
	Type        string
	BackendPort uint32
	Healthy     int
	Unhealthy   int
	Interval    time.Duration
	Timeout     time.Duration
}

// ConfigureHealthCheckResponse is the rpc wrapper for ConfigureHealthCheck response
type ConfigureHealthCheckResponse struct {
	Type   string
	Result string
}

// RegisterBackendCheckRequest is the rpc wrapper for RegisterBackend request
type RegisterBackendCheckRequest struct {
	Type string
	ID   string
	More []string
}

// RegisterBackendCheckResponse is the rpc wrapper for RegisterBackend response
type RegisterBackendCheckResponse struct {
	Type   string
	Result string
}

// DeregisterBackendCheckRequest is the rpc wrapper for DeregisterBackend request
type DeregisterBackendCheckRequest struct {
	Type string
	ID   string
	More []string
}

// DeregisterBackendCheckResponse is the rpc wrapper for DeregisterBackend response
type DeregisterBackendCheckResponse struct {
	Type   string
	Result string
}
