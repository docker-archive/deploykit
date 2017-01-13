package instance

import (
	"encoding/json"

	"github.com/docker/infrakit/pkg/spi/instance"
)

// ValidateRequest is the rpc wrapper for the Validate method args
type ValidateRequest struct {
	Type       string
	Properties *json.RawMessage
}

// ValidateResponse is the rpc wrapper for the Validate response values
type ValidateResponse struct {
	Type string
	OK   bool
}

// ProvisionRequest is the rpc wrapper for Provision request
type ProvisionRequest struct {
	Type string
	Spec instance.Spec
}

// ProvisionResponse is the rpc wrapper for Provision response
type ProvisionResponse struct {
	Type string
	ID   *instance.ID
}

// DestroyRequest is the rpc wrapper for Destroy request
type DestroyRequest struct {
	Type     string
	Instance instance.ID
}

// DestroyResponse is the rpc wrapper for Destroy response
type DestroyResponse struct {
	Type string
	OK   bool
}

// DescribeInstancesRequest is the rpc wrapper for DescribeInstances request
type DescribeInstancesRequest struct {
	Type string
	Tags map[string]string
}

// DescribeInstancesResponse is the rpc wrapper for the DescribeInstances response
type DescribeInstancesResponse struct {
	Type         string
	Descriptions []instance.Description
}
