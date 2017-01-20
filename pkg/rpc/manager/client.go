package manager

import (
	"github.com/docker/infrakit/pkg/manager"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
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
func (c client) IsLeader() bool {
	req := IsLeaderRequest{}
	resp := IsLeaderResponse{}
	c.client.Call("Manager.IsLeader", req, &resp)
	return resp.Leader
}
