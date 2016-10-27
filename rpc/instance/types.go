package instance

import (
	"encoding/json"

	"github.com/docker/infrakit/spi/instance"
)

// ValidateRequest is the rpc wrapper for the Validate method args
type ValidateRequest struct {
	Properties json.RawMessage
}

// ValidateResponse is the rpc wrapper for the Validate response values
type ValidateResponse struct {
	OK bool
}

// ProvisionRequest is the rpc wrapper for Provision request
type ProvisionRequest struct {
	Spec instance.Spec
}

// ProvisionResponse is the rpc wrapper for Provision response
type ProvisionResponse struct {
	ID *instance.ID
}

// DestroyRequest is the rpc wrapper for Destroy request
type DestroyRequest struct {
	Instance instance.ID
}

// DestroyResponse is the rpc wrapper for Destroy response
type DestroyResponse struct {
	OK bool
}

// DescribeInstancesRequest is the rpc wrapper for DescribeInstances request
type DescribeInstancesRequest struct {
	Tags map[string]string
}

// DescribeInstancesResponse is the rpc wrapper for the DescribeInstances response
type DescribeInstancesResponse struct {
	Descriptions []instance.Description
}

// RPCService is the interface exposed via JSON RPC
type RPCService interface {

	// Validate performs validation on the input
	Validate(req *ValidateRequest, resp *ValidateResponse) error

	// Provision creates a new instance based on the spec.
	Provision(req *ProvisionRequest, resp *ProvisionResponse) error

	// Destroy terminates an existing instance.
	Destroy(req *DestroyRequest, resp *DestroyResponse) error

	// DescribeInstances returns descriptions of all instances matching all of the provided tags.
	DescribeInstances(req *DescribeInstancesRequest, resp *DescribeInstancesResponse) error
}
