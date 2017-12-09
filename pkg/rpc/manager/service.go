package manager

import (
	"net/http"
	"net/url"

	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/types"
)

// PluginServer returns a Manager that conforms to the net/rpc rpc call convention.
func PluginServer(p stack.Interface) *Manager {
	return &Manager{manager: p}
}

// Manager is the exported type for json-rpc
type Manager struct {
	manager stack.Interface
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Manager) ImplementedInterface() spi.InterfaceSpec {
	return stack.InterfaceSpec
}

// Objects returns the objects exposed by this kind of RPC service
func (p *Manager) Objects() []rpc.Object {
	return []rpc.Object{{Name: "."}} // no objects
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

// EnforceRequest is the rpc request
type EnforceRequest struct {
	Specs []types.Spec
}

// EnforceResponse is the rpc response
type EnforceResponse struct {
}

// Enforce is the rpc method for Manager.Enforce
func (p *Manager) Enforce(_ *http.Request, req *EnforceRequest, resp *EnforceResponse) error {
	return p.manager.Enforce(req.Specs)
}

// SpecsRequest is the rpc request
type SpecsRequest struct {
}

// SpecsResponse is the rpc response
type SpecsResponse struct {
	Specs []types.Spec
}

// Specs is the rpc method for Manager.Specs
func (p *Manager) Specs(_ *http.Request, req *SpecsRequest, resp *SpecsResponse) error {
	specs, err := p.manager.Specs()
	if err != nil {
		return err
	}
	resp.Specs = specs
	return nil
}

// InspectRequest is the rpc request
type InspectRequest struct {
}

// InspectResponse is the rpc response
type InspectResponse struct {
	Objects []types.Object
}

// Inspect is the rpc method for Manager.Inspect
func (p *Manager) Inspect(_ *http.Request, req *InspectRequest, resp *InspectResponse) error {
	objects, err := p.manager.Inspect()
	if err != nil {
		return err
	}
	resp.Objects = objects
	return nil
}

// TerminateRequest is the rpc request
type TerminateRequest struct {
	Specs []types.Spec
}

// TerminateResponse is the rpc response
type TerminateResponse struct {
}

// Terminate is the rpc method for Manager.Terminate
func (p *Manager) Terminate(_ *http.Request, req *TerminateRequest, resp *TerminateResponse) error {
	return p.manager.Terminate(req.Specs)
}
