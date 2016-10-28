package group

import (
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/docker/infrakit/spi/group"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(protocol, addr string) (group.Plugin, error) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		return nil, err
	}
	return &client{rpc: jsonrpc.NewClient(conn)}, nil
}

type client struct {
	rpc *rpc.Client
}

func (c *client) WatchGroup(grp group.Spec) error {
	req := &WatchGroupRequest{Spec: grp}
	resp := &WatchGroupResponse{}
	err := c.rpc.Call("Group.WatchGroup", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) UnwatchGroup(id group.ID) error {
	req := &UnwatchGroupRequest{ID: id}
	resp := &UnwatchGroupResponse{}
	err := c.rpc.Call("Group.UnwatchGroup", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) InspectGroup(id group.ID) (group.Description, error) {
	req := &InspectGroupRequest{ID: id}
	resp := &InspectGroupResponse{}
	err := c.rpc.Call("Group.InspectGroup", req, resp)
	return resp.Description, err
}

func (c *client) DescribeUpdate(updated group.Spec) (string, error) {
	req := &DescribeUpdateRequest{Spec: updated}
	resp := &DescribeUpdateResponse{}
	err := c.rpc.Call("Group.DescribeUpdate", req, resp)
	return resp.Plan, err
}

func (c *client) UpdateGroup(updated group.Spec) error {
	req := &UpdateGroupRequest{Spec: updated}
	resp := &UpdateGroupResponse{}
	err := c.rpc.Call("Group.UpdateGroup", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) StopUpdate(id group.ID) error {
	req := &StopUpdateRequest{ID: id}
	resp := &StopUpdateResponse{}
	err := c.rpc.Call("Group.StopUpdate", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) DestroyGroup(id group.ID) error {
	req := &DestroyGroupRequest{ID: id}
	resp := &DestroyGroupResponse{}
	err := c.rpc.Call("Group.DestroyGroup", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) DescribeGroups() ([]group.Spec, error) {
	req := &DescribeGroupsRequest{}
	resp := &DescribeGroupsResponse{}
	err := c.rpc.Call("Group.DescribeGroups", req, resp)
	return resp.Groups, err
}
