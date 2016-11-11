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

func (c *client) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	req := &CommitGroupRequest{Spec: grp, Pretend: pretend}
	resp := &CommitGroupResponse{}
	err := c.rpc.Call("Group.CommitGroup", req, resp)
	if err != nil {
		return resp.Details, err
	}
	return resp.Details, nil
}

func (c *client) ReleaseGroup(id group.ID) error {
	req := &ReleaseGroupRequest{ID: id}
	resp := &ReleaseGroupResponse{}
	err := c.rpc.Call("Group.ReleaseGroup", req, resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

func (c *client) DescribeGroup(id group.ID) (group.Description, error) {
	req := &DescribeGroupRequest{ID: id}
	resp := &DescribeGroupResponse{}
	err := c.rpc.Call("Group.DescribeGroup", req, resp)
	return resp.Description, err
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

func (c *client) InspectGroups() ([]group.Spec, error) {
	req := &InspectGroupsRequest{}
	resp := &InspectGroupsResponse{}
	err := c.rpc.Call("Group.InspectGroups", req, resp)
	return resp.Groups, err
}
