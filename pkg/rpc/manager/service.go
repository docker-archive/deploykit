package manager

import (
	"net/http"

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
