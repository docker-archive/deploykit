package instance

import (
	"encoding/json"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// NewClient returns a plugin interface implementation connected to a plugin
func NewClient(socketPath string) instance.Plugin {
	return &client{client: rpc_client.New(socketPath, instance.InterfaceSpec)}
}

type client struct {
	client rpc_client.Client
}

// Validate performs local validation on a provision request.
func (c client) Validate(properties json.RawMessage) error {
	req := ValidateRequest{Properties: &properties}
	resp := ValidateResponse{}

	return c.client.Call("Instance.Validate", req, &resp)
}

// Provision creates a new instance based on the spec.
func (c client) Provision(spec instance.Spec) (*instance.ID, error) {
	req := ProvisionRequest{Spec: spec}
	resp := ProvisionResponse{}

	if err := c.client.Call("Instance.Provision", req, &resp); err != nil {
		return nil, err
	}

	return resp.ID, nil
}

// Destroy terminates an existing instance.
func (c client) Destroy(instance instance.ID) error {
	req := DestroyRequest{Instance: instance}
	resp := DestroyResponse{}

	return c.client.Call("Instance.Destroy", req, &resp)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (c client) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	req := DescribeInstancesRequest{Tags: tags}
	resp := DescribeInstancesResponse{}

	err := c.client.Call("Instance.DescribeInstances", req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Descriptions, nil
}
