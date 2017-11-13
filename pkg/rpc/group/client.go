package group

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(name plugin.Name, socketPath string) (group.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, group.InterfaceSpec)
	if err != nil {
		return nil, err
	}

	return Adapt(name, rpcClient), nil
}

// Adapt returns a group Plugin implementation based on given rpc client.  Assumption here is that
// the rpcClient has been verified to support the group plugin RPC interface.
func Adapt(name plugin.Name, rpcClient rpc_client.Client) group.Plugin {
	return &client{name: name, client: rpcClient}
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

func (c client) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	req := CommitGroupRequest{Name: c.name, Spec: grp, Pretend: pretend}
	resp := CommitGroupResponse{}
	err := c.client.Call("Group.CommitGroup", req, &resp)
	return resp.Details, err
}

func (c client) FreeGroup(id group.ID) error {
	req := FreeGroupRequest{Name: c.name, ID: id}
	resp := FreeGroupResponse{}
	return c.client.Call("Group.FreeGroup", req, &resp)
}

func (c client) DescribeGroup(id group.ID) (group.Description, error) {
	req := DescribeGroupRequest{Name: c.name, ID: id}
	resp := DescribeGroupResponse{}
	err := c.client.Call("Group.DescribeGroup", req, &resp)
	return resp.Description, err
}

func (c client) DestroyGroup(id group.ID) error {
	req := DestroyGroupRequest{Name: c.name, ID: id}
	resp := DestroyGroupResponse{}
	return c.client.Call("Group.DestroyGroup", req, &resp)
}

func (c client) InspectGroups() ([]group.Spec, error) {
	req := InspectGroupsRequest{Name: c.name}
	resp := InspectGroupsResponse{}
	err := c.client.Call("Group.InspectGroups", req, &resp)
	return resp.Groups, err
}

func (c client) DestroyInstances(id group.ID, instances []instance.ID) error {
	req := DestroyInstancesRequest{
		Name:      c.name,
		ID:        id,
		Instances: instances,
	}
	resp := DestroyInstancesResponse{}
	return c.client.Call("Group.DestroyInstances", req, &resp)
}

func (c client) Size(id group.ID) (int, error) {
	req := SizeRequest{
		Name: c.name,
		ID:   id,
	}
	resp := SizeResponse{}
	err := c.client.Call("Group.Size", req, &resp)
	return resp.Size, err
}

func (c client) SetSize(id group.ID, size int) error {
	req := SetSizeRequest{
		Name: c.name,
		ID:   id,
		Size: size,
	}
	resp := SetSizeResponse{}
	return c.client.Call("Group.SetSize", req, &resp)
}
