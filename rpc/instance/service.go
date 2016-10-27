package instance

import (
	"github.com/docker/infrakit/spi/instance"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p instance.Plugin) RPCService {
	return &Instance{plugin: p}
}

// Instance is the JSON RPC service representing the Instance Plugin.  It must be exported in order to be
// registered by the rpc server package.
type Instance struct {
	plugin instance.Plugin
}

// Validate performs local validation on a provision request.
func (p *Instance) Validate(req *ValidateRequest, resp *ValidateResponse) error {
	err := p.plugin.Validate(req.Properties)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Provision creates a new instance based on the spec.
func (p *Instance) Provision(req *ProvisionRequest, resp *ProvisionResponse) error {
	id, err := p.plugin.Provision(req.Spec)
	if err != nil {
		return err
	}
	resp.ID = id
	return nil
}

// Destroy terminates an existing instance.
func (p *Instance) Destroy(req *DestroyRequest, resp *DestroyResponse) error {
	err := p.plugin.Destroy(req.Instance)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *Instance) DescribeInstances(req *DescribeInstancesRequest, resp *DescribeInstancesResponse) error {
	desc, err := p.plugin.DescribeInstances(req.Tags)
	if err != nil {
		return err
	}
	resp.Descriptions = desc
	return nil
}
