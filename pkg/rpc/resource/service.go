package resource

import (
	"net/http"

	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/types"
)

// PluginServer returns an RPCService that conforms to the net/rpc calling convention.
func PluginServer(p resource.Plugin) *Resource {
	return &Resource{plugin: p}
}

// Resource is the exported type needed to conform to the json-rpc calling convention.
type Resource struct {
	plugin resource.Plugin
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See plugin.Vendor.
func (p *Resource) VendorInfo() *spi.VendorInfo {
	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ExampleProperties returns an example properties used by the plugin.
func (p *Resource) ExampleProperties() *types.Any {
	if i, is := p.plugin.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Resource) ImplementedInterface() spi.InterfaceSpec {
	return resource.InterfaceSpec
}

// Objects returns the objects exposed by this service (or kind/ category)
func (p *Resource) Objects() []rpc.Object {
	return []rpc.Object{{Name: "."}}
}

// Commit is the rpc method to commit resources.
func (p *Resource) Commit(_ *http.Request, req *CommitRequest, resp *CommitResponse) error {
	details, err := p.plugin.Commit(req.Spec, req.Pretend)
	if err != nil {
		return err
	}
	resp.Details = details
	return nil
}

// Destroy is the rpc method to destroy resources.
func (p *Resource) Destroy(_ *http.Request, req *DestroyRequest, resp *DestroyResponse) error {
	details, err := p.plugin.Destroy(req.Spec, req.Pretend)
	if err != nil {
		return err
	}
	resp.Details = details
	return nil
}

// DescribeResources is the rpc method to describe a resource.
func (p *Resource) DescribeResources(_ *http.Request, req *DescribeResourcesRequest, resp *DescribeResourcesResponse) error {
	details, err := p.plugin.DescribeResources(req.Spec)
	if err != nil {
		return err
	}
	resp.Details = details
	return nil
}
