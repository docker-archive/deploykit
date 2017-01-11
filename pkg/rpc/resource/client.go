package resource

import (
	"encoding/json"

	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/resource"
)

// NewClient returns a plugin interface implementation connected to a plugin
func NewClient(socketPath string) resource.Plugin {
	return &client{client: rpc_client.New(socketPath, resource.InterfaceSpec)}
}

type client struct {
	client rpc_client.Client
}

// Validate performs local validation on a provision request.
func (c client) Validate(resourceType string, properties json.RawMessage) error {
	req := ValidateRequest{Type: resourceType, Properties: &properties}
	resp := ValidateResponse{}

	return c.client.Call("Resource.Validate", req, &resp)
}

// Provision creates a new resource based on the spec.
func (c client) Provision(spec resource.Spec) (*resource.ID, error) {
	req := ProvisionRequest{Spec: spec}
	resp := ProvisionResponse{}

	if err := c.client.Call("Resource.Provision", req, &resp); err != nil {
		return nil, err
	}

	return resp.ID, nil
}

// Destroy terminates an existing resource.
func (c client) Destroy(resourceType string, resource resource.ID) error {
	req := DestroyRequest{Type: resourceType, Resource: resource}
	resp := DestroyResponse{}

	return c.client.Call("Resource.Destroy", req, &resp)
}

// DescribeResources returns descriptions of all resources matching the given type and all of the provided tags.
func (c client) DescribeResources(resourceType string, tags map[string]string) ([]resource.Description, error) {
	req := DescribeResourcesRequest{Type: resourceType, Tags: tags}
	resp := DescribeResourcesResponse{}

	err := c.client.Call("Resource.DescribeResources", req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Descriptions, nil
}
