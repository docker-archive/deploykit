package flavor

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(name plugin.Name, socketPath string) (flavor.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, flavor.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{name: name, client: rpcClient}, nil
}

// Adapt converts a rpc client to a Plugin object
func Adapt(name plugin.Name, rpcClient rpc_client.Client) flavor.Plugin {
	return &client{name: name, client: rpcClient}
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

// Validate checks whether the helper can support a configuration.
func (c client) Validate(flavorProperties *types.Any, allocation group.AllocationMethod) error {
	_, flavorType := c.name.GetLookupAndType()
	req := ValidateRequest{Type: flavorType, Properties: flavorProperties, Allocation: allocation}
	resp := ValidateResponse{}
	return c.client.Call("Flavor.Validate", req, &resp)
}

// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
// the flavor configuration.
func (c client) Prepare(flavorProperties *types.Any, spec instance.Spec,
	allocation group.AllocationMethod, index group.Index) (instance.Spec, error) {

	_, flavorType := c.name.GetLookupAndType()
	req := PrepareRequest{Type: flavorType, Properties: flavorProperties, Spec: spec, Allocation: allocation, Index: index}
	resp := PrepareResponse{}
	err := c.client.Call("Flavor.Prepare", req, &resp)
	if err != nil {
		return spec, err
	}
	return resp.Spec, nil
}

// Healthy determines the Health of this Flavor on an instance.
func (c client) Healthy(flavorProperties *types.Any, inst instance.Description) (flavor.Health, error) {

	_, flavorType := c.name.GetLookupAndType()
	req := HealthyRequest{Type: flavorType, Properties: flavorProperties, Instance: inst}
	resp := HealthyResponse{}
	err := c.client.Call("Flavor.Healthy", req, &resp)
	return resp.Health, err
}

// Drain allows the flavor to perform a best-effort cleanup operation before the instance is destroyed.
func (c client) Drain(flavorProperties *types.Any, inst instance.Description) error {

	_, flavorType := c.name.GetLookupAndType()
	req := DrainRequest{Type: flavorType, Properties: flavorProperties, Instance: inst}
	resp := DrainResponse{}
	err := c.client.Call("Flavor.Drain", req, &resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}
