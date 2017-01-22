package group

import (
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/group"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(socketPath string) (group.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, group.InterfaceSpec)
	if err != nil {
		return nil, err
	}

	return Adapt(rpcClient), nil
}

// Adapt returns a group Plugin implementation based on given rpc client.  Assumption here is that
// the rpcClient has been verified to support the group plugin RPC interface.
func Adapt(rpcClient rpc_client.Client) group.Plugin {
	return &client{client: rpcClient}
}

type client struct {
	client rpc_client.Client
}

func (c client) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	req := CommitGroupRequest{Spec: grp, Pretend: pretend}
	resp := CommitGroupResponse{}
	err := c.client.Call("Group.CommitGroup", req, &resp)
	if err != nil {
		return resp.Details, err
	}
	return resp.Details, nil
}

func (c client) FreeGroup(id group.ID) error {
	req := FreeGroupRequest{ID: id}
	resp := FreeGroupResponse{}
	err := c.client.Call("Group.FreeGroup", req, &resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c client) DescribeGroup(id group.ID) (group.Description, error) {
	req := DescribeGroupRequest{ID: id}
	resp := DescribeGroupResponse{}
	err := c.client.Call("Group.DescribeGroup", req, &resp)
	return resp.Description, err
}

func (c client) DestroyGroup(id group.ID) error {
	req := DestroyGroupRequest{ID: id}
	resp := DestroyGroupResponse{}
	err := c.client.Call("Group.DestroyGroup", req, &resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c client) InspectGroups() ([]group.Spec, error) {
	req := InspectGroupsRequest{}
	resp := InspectGroupsResponse{}
	err := c.client.Call("Group.InspectGroups", req, &resp)
	return resp.Groups, err
}
