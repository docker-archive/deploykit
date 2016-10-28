package group

import (
	"github.com/docker/infrakit/spi/group"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p group.Plugin) RPCService {
	return &Group{plugin: p}
}

// Group the exported type needed to conform to json-rpc call convention
type Group struct {
	plugin group.Plugin
}

// WatchGroup is the rpc method to watch a group
func (p *Group) WatchGroup(req *WatchGroupRequest, resp *WatchGroupResponse) error {
	err := p.plugin.WatchGroup(req.Spec)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// UnwatchGroup is the rpc method to unwatch a group
func (p *Group) UnwatchGroup(req *UnwatchGroupRequest, resp *UnwatchGroupResponse) error {
	err := p.plugin.UnwatchGroup(req.ID)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// InspectGroup is the rpc method to inspect a group
func (p *Group) InspectGroup(req *InspectGroupRequest, resp *InspectGroupResponse) error {
	desc, err := p.plugin.InspectGroup(req.ID)
	if err != nil {
		return err
	}
	resp.Description = desc
	return nil
}

// DescribeUpdate is the rpc method to describe an update without performing it
func (p *Group) DescribeUpdate(req *DescribeUpdateRequest, resp *DescribeUpdateResponse) error {
	plan, err := p.plugin.DescribeUpdate(req.Spec)
	if err != nil {
		return err
	}
	resp.Plan = plan
	return nil
}

// UpdateGroup is the rpc method to actually updating a group
func (p *Group) UpdateGroup(req *UpdateGroupRequest, resp *UpdateGroupResponse) error {
	err := p.plugin.UpdateGroup(req.Spec)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// StopUpdate is the rpc method to stop a current update
func (p *Group) StopUpdate(req *StopUpdateRequest, resp *StopUpdateResponse) error {
	err := p.plugin.StopUpdate(req.ID)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// DestroyGroup is the rpc method to destroy a group
func (p *Group) DestroyGroup(req *DestroyGroupRequest, resp *DestroyGroupResponse) error {
	err := p.plugin.DestroyGroup(req.ID)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// DescribeGroups is the rpc method to describe groups
func (p *Group) DescribeGroups(req *DescribeGroupsRequest, resp *DescribeGroupsResponse) error {
	groups, err := p.plugin.DescribeGroups()
	if err != nil {
		return err
	}
	resp.Groups = groups
	return nil
}
