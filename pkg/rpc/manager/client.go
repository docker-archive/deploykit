package manager

import (
	"net/url"

	"github.com/docker/infrakit/pkg/manager"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(socketPath string) (manager.Manager, error) {
	rpcClient, err := rpc_client.New(socketPath, manager.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return Adapt(rpcClient), nil
}

// Adapt converts a rpc client to a Manager object
func Adapt(rpcClient rpc_client.Client) manager.Manager {
	return &client{client: rpcClient}
}

type client struct {
	client rpc_client.Client
}

// IsLeader returns true if the maanger is a leader
func (c client) IsLeader() (bool, error) {
	req := IsLeaderRequest{}
	resp := IsLeaderResponse{}
	err := c.client.Call("Manager.IsLeader", req, &resp)
	return resp.Leader, err
}

// LeaderLocation returns the location of the leader
func (c client) LeaderLocation() (*url.URL, error) {
	req := LeaderLocationRequest{}
	resp := LeaderLocationResponse{}
	err := c.client.Call("Manager.LeaderLocation", req, &resp)
	return resp.Location, err
}

// Enforce enforces infrastructure state to match that of the specs
func (c client) Enforce(specs []types.Spec) error {
	req := EnforceRequest{
		Specs: specs,
	}
	resp := EnforceResponse{}
	err := c.client.Call("Manager.Enforce", req, &resp)
	return err
}

// Inspect returns the current state of the infrastructure
func (c client) Inspect() ([]types.Object, error) {
	req := InspectRequest{}
	resp := InspectResponse{}
	err := c.client.Call("Manager.Inspect", req, &resp)
	return resp.Objects, err
}

// Terminate destroys all resources associated with the specs
func (c client) Terminate(specs []types.Spec) error {
	req := TerminateRequest{
		Specs: specs,
	}
	resp := TerminateResponse{}
	err := c.client.Call("Manager.Terminate", req, &resp)
	return err
}
