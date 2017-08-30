package loadbalancer

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// NameRequest is the rpc wrapper for the Name request
type NameRequest struct {
	Type string
}

// Plugin implements pkg/rpc/internal/Addressable
func (r NameRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r RoutesRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r PublishRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r UnpublishRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r ConfigureHealthCheckRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r RegisterBackendsRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r DeregisterBackendsRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r BackendsRequest) Plugin() (plugin.Name, error) {
	return plugin.Name(fmt.Sprintf("./%v", r.Type)), nil
}

// BackendsResponse is the rpc response for Backends call
type BackendsResponse struct {
	Type string
	IDs  []instance.ID
}
