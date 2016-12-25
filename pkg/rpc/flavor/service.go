package flavor

import (
	"encoding/json"
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/flavor"
)

// PluginServer returns a Flavor that conforms to the net/rpc rpc call convention.
func PluginServer(p flavor.Plugin) *Flavor {
	return &Flavor{plugin: p}
}

// Flavor the exported type needed to conform to json-rpc call convention
type Flavor struct {
	plugin flavor.Plugin
}

// Info returns a metadata object about the plugin, if the plugin implements it.  See spi.Vendor
func (p *Flavor) VendorInfo() *spi.VendorInfo {
	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// SetExampleProperties sets the rpc request with any example properties/ custom type
func (p *Flavor) SetExampleProperties(request interface{}) {
	i, is := p.plugin.(spi.InputExample)
	if !is {
		return
	}
	example := i.ExampleProperties()
	if example == nil {
		return
	}

	switch request := request.(type) {
	case *PrepareRequest:
		request.Properties = example
	case *HealthyRequest:
		request.Properties = example
	case *DrainRequest:
		request.Properties = example
	}
}

// exampleProperties returns an example properties used by the plugin
func (p *Flavor) exampleProperties() *json.RawMessage {
	if i, is := p.plugin.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Flavor) ImplementedInterface() spi.InterfaceSpec {
	return flavor.InterfaceSpec
}

// Validate checks whether the helper can support a configuration.
func (p *Flavor) Validate(_ *http.Request, req *ValidateRequest, resp *ValidateResponse) error {
	err := p.plugin.Validate(*req.Properties, req.Allocation)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
// the flavor configuration.
func (p *Flavor) Prepare(_ *http.Request, req *PrepareRequest, resp *PrepareResponse) error {
	spec, err := p.plugin.Prepare(*req.Properties, req.Spec, req.Allocation)
	if err != nil {
		return err
	}
	resp.Spec = spec
	return nil
}

// Healthy determines whether an instance is healthy.
func (p *Flavor) Healthy(_ *http.Request, req *HealthyRequest, resp *HealthyResponse) error {
	health, err := p.plugin.Healthy(*req.Properties, req.Instance)
	if err != nil {
		return err
	}
	resp.Health = health
	return nil
}

// Drain drains the instance. It's the inverse of prepare before provision and happens before destroy.
func (p *Flavor) Drain(_ *http.Request, req *DrainRequest, resp *DrainResponse) error {
	err := p.plugin.Drain(*req.Properties, req.Instance)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}
