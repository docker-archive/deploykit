package resource

import (
	"errors"
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/resource"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p resource.Plugin) *Resource {
	return &Resource{plugin: p}
}

// Resource is the JSON RPC service representing the Resource Plugin.  It must be exported in order to be
// registered by the rpc server package.
type Resource struct {
	plugin resource.Plugin
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.
func (p *Resource) VendorInfo() *spi.VendorInfo {
	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Resource) ImplementedInterface() spi.InterfaceSpec {
	return resource.InterfaceSpec
}

// Validate performs local validation on a provision request.
func (p *Resource) Validate(_ *http.Request, req *ValidateRequest, resp *ValidateResponse) error {
	if req.Properties == nil {
		return errors.New("Request Properties must be set")
	}

	err := p.plugin.Validate(req.Type, *req.Properties)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Provision creates a new resource based on the spec.
func (p *Resource) Provision(_ *http.Request, req *ProvisionRequest, resp *ProvisionResponse) error {
	id, err := p.plugin.Provision(req.Spec)
	if err != nil {
		return err
	}
	resp.ID = id
	return nil
}

// Destroy terminates an existing resource.
func (p *Resource) Destroy(_ *http.Request, req *DestroyRequest, resp *DestroyResponse) error {
	err := p.plugin.Destroy(req.Type, req.Resource)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// DescribeResources returns descriptions of all resources matching all of the provided tags.
func (p *Resource) DescribeResources(_ *http.Request, req *DescribeResourcesRequest, resp *DescribeResourcesResponse) error {
	desc, err := p.plugin.DescribeResources(req.Type, req.Tags)
	if err != nil {
		return err
	}
	resp.Descriptions = desc
	return nil
}
