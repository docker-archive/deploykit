package resource

import (
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/resource"
)

// NewClient returns a plugin interface implementation connected to a remote plugin.
func NewClient(socketPath string) (resource.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, resource.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return Adapt(rpcClient), nil
}

// Adapt converts an RPC client to a resource plugin.
func Adapt(rpcClient rpc_client.Client) resource.Plugin {
	return &client{client: rpcClient}
}

type client struct {
	client rpc_client.Client
}

func (c client) Commit(spec resource.Spec, pretend bool) (string, error) {
	req := CommitRequest{Spec: spec, Pretend: pretend}
	resp := CommitResponse{}
	err := c.client.Call("Resource.Commit", req, &resp)
	if err != nil {
		return resp.Details, err
	}
	return resp.Details, nil
}

func (c client) Destroy(spec resource.Spec, pretend bool) (string, error) {
	req := DestroyRequest{Spec: spec, Pretend: pretend}
	resp := DestroyResponse{}
	err := c.client.Call("Resource.Destroy", req, &resp)
	if err != nil {
		return resp.Details, err
	}
	return resp.Details, nil
}

func (c client) DescribeResources(spec resource.Spec) (string, error) {
	req := DescribeResourcesRequest{Spec: spec}
	resp := DescribeResourcesResponse{}
	err := c.client.Call("Resource.DescribeResources", req, &resp)
	if err != nil {
		return "", err
	}
	return resp.Details, nil
}
