package group

import (
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// PluginServerWithGroups returns a Group map of plugins by group.ID
func PluginServerWithGroups(list func() (map[group.ID]group.Plugin, error)) *Group {
	keyed := internal.ServeKeyed(

		// This is where templates would be nice...
		func() (map[string]interface{}, error) {
			m, err := list()
			if err != nil {
				return nil, err
			}
			out := map[string]interface{}{}
			for k, v := range m {
				out[string(k)] = v
			}
			return out, nil
		},
	)

	return &Group{
		keyed: keyed,
	}
}

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p group.Plugin) *Group {
	return &Group{keyed: internal.ServeSingle(p)}
}

// Group the exported type needed to conform to json-rpc call convention
type Group struct {
	keyed *internal.Keyed
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See plugin.Vendor
func (p *Group) VendorInfo() *spi.VendorInfo {
	base, _ := p.keyed.Keyed(plugin.Name("."))
	if m, is := base.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ExampleProperties returns an example properties used by the plugin
func (p *Group) ExampleProperties() *types.Any {
	base, _ := p.keyed.Keyed(plugin.Name("."))
	if i, is := base.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Group) ImplementedInterface() spi.InterfaceSpec {
	return group.InterfaceSpec

}

// Objects returns the objects exposed by this kind of RPC service
func (p *Group) Objects() []rpc.Object {
	return p.keyed.Objects()
}

// CommitGroup is the rpc method to commit a group
func (p *Group) CommitGroup(_ *http.Request, req *CommitGroupRequest, resp *CommitGroupResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		details, err := v.(group.Plugin).CommitGroup(req.Spec, req.Pretend)
		if err != nil {
			return err
		}
		resp.Details = details
		resp.ID = req.Spec.ID
		return nil
	})
}

// FreeGroup is the rpc method to free a group
func (p *Group) FreeGroup(_ *http.Request, req *FreeGroupRequest, resp *FreeGroupResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		err := v.(group.Plugin).FreeGroup(req.ID)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		return nil
	})
}

// DescribeGroup is the rpc method to describe a group
func (p *Group) DescribeGroup(_ *http.Request, req *DescribeGroupRequest, resp *DescribeGroupResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		desc, err := v.(group.Plugin).DescribeGroup(req.ID)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		resp.Description = desc
		return nil
	})
}

// DestroyGroup is the rpc method to destroy a group
func (p *Group) DestroyGroup(_ *http.Request, req *DestroyGroupRequest, resp *DestroyGroupResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		err := v.(group.Plugin).DestroyGroup(req.ID)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		return nil
	})
}

// InspectGroups is the rpc method to inspect groups
func (p *Group) InspectGroups(_ *http.Request, req *InspectGroupsRequest, resp *InspectGroupsResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		groups, err := v.(group.Plugin).InspectGroups()
		if err != nil {
			return err
		}
		resp.Groups = groups
		return nil
	})
}

// DestroyInstances is the rpc method to destroy specific instances
func (p *Group) DestroyInstances(_ *http.Request, req *DestroyInstancesRequest, resp *DestroyInstancesResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		err := v.(group.Plugin).DestroyInstances(req.ID, req.Instances)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		return nil
	})
}

// Size is the rpc method to get the group target size
func (p *Group) Size(_ *http.Request, req *SizeRequest, resp *SizeResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		size, err := v.(group.Plugin).Size(req.ID)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		resp.Size = size
		return nil
	})
}

// SetSize is the rpc method to set the group target size
func (p *Group) SetSize(_ *http.Request, req *SetSizeRequest, resp *SetSizeResponse) error {
	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		err := v.(group.Plugin).SetSize(req.ID, req.Size)
		if err != nil {
			return err
		}
		resp.ID = req.ID
		return nil
	})
}
