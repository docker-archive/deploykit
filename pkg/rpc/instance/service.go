package instance

import (
	"errors"
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p instance.Plugin) *Instance {
	return &Instance{plugin: p}
}

// Instance is the JSON RPC service representing the Instance Plugin.  It must be exported in order to be
// registered by the rpc server package.
type Instance struct {
	plugin instance.Plugin
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.
func (p *Instance) VendorInfo() *spi.VendorInfo {
	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// SetExampleProperties sets the rpc request with any example properties/ custom type
func (p *Instance) SetExampleProperties(request interface{}) {
	i, is := p.plugin.(spi.InputExample)
	if !is {
		return
	}
	example := i.ExampleProperties()
	if example == nil {
		return
	}

	switch request := request.(type) {
	case *ValidateRequest:
		request.Properties = example
	case *ProvisionRequest:
		request.Spec.Properties = example
	}
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Instance) ImplementedInterface() spi.InterfaceSpec {
	return instance.InterfaceSpec
}

// Validate performs local validation on a provision request.
func (p *Instance) Validate(_ *http.Request, req *ValidateRequest, resp *ValidateResponse) error {
	if req.Properties == nil {
		return errors.New("Request Properties must be set")
	}

	err := p.plugin.Validate(*req.Properties)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Provision creates a new instance based on the spec.
func (p *Instance) Provision(_ *http.Request, req *ProvisionRequest, resp *ProvisionResponse) error {
	id, err := p.plugin.Provision(req.Spec)
	if err != nil {
		return err
	}
	resp.ID = id
	return nil
}

// Destroy terminates an existing instance.
func (p *Instance) Destroy(_ *http.Request, req *DestroyRequest, resp *DestroyResponse) error {
	err := p.plugin.Destroy(req.Instance)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *Instance) DescribeInstances(_ *http.Request, req *DescribeInstancesRequest, resp *DescribeInstancesResponse) error {
	desc, err := p.plugin.DescribeInstances(req.Tags)
	if err != nil {
		return err
	}
	resp.Descriptions = desc
	return nil
}
