package flavor

import (
	"github.com/docker/infrakit/spi/flavor"
)

// PluginServer returns a RPCService that conforms to the net/rpc rpc call convention.
func PluginServer(p flavor.Plugin) RPCService {
	return &Flavor{plugin: p}
}

// Flavor the exported type needed to conform to json-rpc call convention
type Flavor struct {
	plugin flavor.Plugin
}

// Validate checks whether the helper can support a configuration.
func (p *Flavor) Validate(req *ValidateRequest, resp *ValidateResponse) error {
	err := p.plugin.Validate(req.Properties, req.Allocation)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
// the flavor configuration.
func (p *Flavor) Prepare(req *PrepareRequest, resp *PrepareResponse) error {
	spec, err := p.plugin.Prepare(req.Properties, req.Spec, req.Allocation)
	if err != nil {
		return err
	}
	resp.Spec = spec
	return nil
}

// Healthy determines whether an instance is healthy.
func (p *Flavor) Healthy(req *HealthyRequest, resp *HealthyResponse) error {
	health, err := p.plugin.Healthy(req.Properties, req.Instance)
	if err != nil {
		return err
	}
	resp.Health = health
	return nil
}

// Drain drains the instance. It's the inverse of prepare before provision and happens before destroy.
func (p *Flavor) Drain(req *DrainRequest, resp *DrainResponse) error {
	err := p.plugin.Drain(req.Properties, req.Instance)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}
