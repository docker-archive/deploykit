package group

import (
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p group.Plugin) *Group {
	return &Group{plugin: p}
}

// Group the exported type needed to conform to json-rpc call convention
type Group struct {
	plugin group.Plugin
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See plugin.Vendor
func (p *Group) VendorInfo() *spi.VendorInfo {
	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ExampleProperties returns an example properties used by the plugin
func (p *Group) ExampleProperties() *types.Any {
	if i, is := p.plugin.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Group) ImplementedInterface() spi.InterfaceSpec {
	return group.InterfaceSpec

}

// Types returns the types exposed by this kind of RPC service
func (p *Group) Types() []string {
	return []string{"."} // no types
}

// CommitGroup is the rpc method to commit a group
func (p *Group) CommitGroup(_ *http.Request, req *CommitGroupRequest, resp *CommitGroupResponse) error {
	details, err := p.plugin.CommitGroup(req.Spec, req.Pretend)
	if err != nil {
		return err
	}
	resp.Details = details
	resp.ID = req.Spec.ID
	return nil
}

// FreeGroup is the rpc method to free a group
func (p *Group) FreeGroup(_ *http.Request, req *FreeGroupRequest, resp *FreeGroupResponse) error {
	err := p.plugin.FreeGroup(req.ID)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	return nil
}

// DescribeGroup is the rpc method to describe a group
func (p *Group) DescribeGroup(_ *http.Request, req *DescribeGroupRequest, resp *DescribeGroupResponse) error {
	desc, err := p.plugin.DescribeGroup(req.ID)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	resp.Description = desc
	return nil
}

// DestroyGroup is the rpc method to destroy a group
func (p *Group) DestroyGroup(_ *http.Request, req *DestroyGroupRequest, resp *DestroyGroupResponse) error {
	err := p.plugin.DestroyGroup(req.ID)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	return nil
}

// InspectGroups is the rpc method to inspect groups
func (p *Group) InspectGroups(_ *http.Request, req *InspectGroupsRequest, resp *InspectGroupsResponse) error {
	groups, err := p.plugin.InspectGroups()
	if err != nil {
		return err
	}
	resp.Groups = groups
	return nil
}

// DestroyInstances is the rpc method to destroy specific instances
func (p *Group) DestroyInstances(_ *http.Request, req *DestroyInstancesRequest, resp *DestroyInstancesResponse) error {
	err := p.plugin.DestroyInstances(req.ID, req.Instances)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	return nil
}

// Size is the rpc method to get the group target size
func (p *Group) Size(_ *http.Request, req *SizeRequest, resp *SizeResponse) error {
	size, err := p.plugin.Size(req.ID)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	resp.Size = size
	return nil
}

// SetSize is the rpc method to set the group target size
func (p *Group) SetSize(_ *http.Request, req *SetSizeRequest, resp *SetSizeResponse) error {
	err := p.plugin.SetSize(req.ID, req.Size)
	if err != nil {
		return err
	}
	resp.ID = req.ID
	return nil
}
