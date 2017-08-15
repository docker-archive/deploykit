package manager

import (
	"net/http"
	"net/url"

	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/spi"
)

// PluginServer returns a Manager that conforms to the net/rpc rpc call convention.
func PluginServer(p manager.Manager) *Manager {
	return &Manager{manager: p}
}

// Manager is the exported type for json-rpc
type Manager struct {
	manager manager.Manager
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Manager) ImplementedInterface() spi.InterfaceSpec {
	return manager.InterfaceSpec
}

// Types returns the types exposed by this kind of RPC service
func (p *Manager) Types() []string {
	return []string{"."} // no types
}

// IsLeaderRequest is the rpc request
type IsLeaderRequest struct {
}

// IsLeaderResponse is the rpc response
type IsLeaderResponse struct {
	Leader bool
}

// IsLeader returns information about leadership status for this manager.
func (p *Manager) IsLeader(_ *http.Request, req *IsLeaderRequest, resp *IsLeaderResponse) error {
	is, err := p.manager.IsLeader()
	if err == nil {
		resp.Leader = is
	}
	return err
}

// LeaderLocationRequest is the rpc request
type LeaderLocationRequest struct {
}

// LeaderLocationResponse is the rpc response
type LeaderLocationResponse struct {
	Location *url.URL
}

// LeaderLocation returns the location of the leader
func (p *Manager) LeaderLocation(_ *http.Request, req *LeaderLocationRequest, resp *LeaderLocationResponse) error {
	u, err := p.manager.LeaderLocation()
	if err == nil {
		resp.Location = u
	}
	return err
}
