package flavor

import (
	"encoding/json"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(socketPath string) flavor.Plugin {
	return &client{client: rpc_client.New(socketPath, flavor.InterfaceSpec)}
}

type client struct {
	client rpc_client.Client
}

// Validate checks whether the helper can support a configuration.
func (c client) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
	req := ValidateRequest{Properties: &flavorProperties, Allocation: allocation}
	resp := ValidateResponse{}
	return c.client.Call("Flavor.Validate", req, &resp)
}

// Prepare allows the Flavor to modify the provisioning instructions for an instance.  For example, a
// helper could be used to place additional tags on the machine, or generate a specialized Init command based on
// the flavor configuration.
func (c client) Prepare(flavorProperties json.RawMessage, spec instance.Spec, allocation types.AllocationMethod) (instance.Spec, error) {
	req := PrepareRequest{Properties: &flavorProperties, Spec: spec, Allocation: allocation}
	resp := PrepareResponse{}
	err := c.client.Call("Flavor.Prepare", req, &resp)
	if err != nil {
		return spec, err
	}
	return resp.Spec, nil
}

// Healthy determines the Health of this Flavor on an instance.
func (c client) Healthy(flavorProperties json.RawMessage, inst instance.Description) (flavor.Health, error) {
	req := HealthyRequest{Properties: &flavorProperties, Instance: inst}
	resp := HealthyResponse{}
	err := c.client.Call("Flavor.Healthy", req, &resp)
	return resp.Health, err
}

// Drain allows the flavor to perform a best-effort cleanup operation before the instance is destroyed.
func (c client) Drain(flavorProperties json.RawMessage, inst instance.Description) error {
	req := DrainRequest{Properties: &flavorProperties, Instance: inst}
	resp := DrainResponse{}
	err := c.client.Call("Flavor.Drain", req, &resp)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}
