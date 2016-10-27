package instance

import (
	"encoding/json"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/docker/infrakit/spi/instance"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(protocol, addr string) (instance.Plugin, error) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		return nil, err
	}
	return &client{rpc: jsonrpc.NewClient(conn)}, nil
}

type client struct {
	rpc *rpc.Client
}

// Validate performs local validation on a provision request.
func (c *client) Validate(properties json.RawMessage) error {
	req := &ValidateRequest{Properties: properties}
	resp := &ValidateResponse{}
	return c.rpc.Call("Instance.Validate", req, resp)
}

// Provision creates a new instance based on the spec.
func (c *client) Provision(spec instance.Spec) (*instance.ID, error) {
	req := &ProvisionRequest{Spec: spec}
	resp := &ProvisionResponse{}
	err := c.rpc.Call("Instance.Provision", req, resp)
	if err != nil {
		return nil, err
	}
	return resp.ID, nil
}

// Destroy terminates an existing instance.
func (c *client) Destroy(instance instance.ID) error {
	req := &DestroyRequest{Instance: instance}
	resp := &DestroyResponse{}
	return c.rpc.Call("Instance.Destroy", req, resp)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (c *client) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	req := &DescribeInstancesRequest{Tags: tags}
	resp := &DescribeInstancesResponse{}
	err := c.rpc.Call("Instance.DescribeInstances", req, resp)
	if err != nil {
		return nil, err
	}
	return resp.Descriptions, nil
}
