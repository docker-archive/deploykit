package resource

import (
	"encoding/json"

	"github.com/docker/infrakit/pkg/spi/resource"
)

// ValidateRequest is the rpc wrapper for the Validate method args
type ValidateRequest struct {
	Type       string
	Properties *json.RawMessage
}

// ValidateResponse is the rpc wrapper for the Validate response values
type ValidateResponse struct {
	OK bool
}

// ProvisionRequest is the rpc wrapper for Provision request
type ProvisionRequest struct {
	Spec resource.Spec
}

// ProvisionResponse is the rpc wrapper for Provision response
type ProvisionResponse struct {
	ID *resource.ID
}

// DestroyRequest is the rpc wrapper for Destroy request
type DestroyRequest struct {
	Type     string
	Resource resource.ID
}

// DestroyResponse is the rpc wrapper for Destroy response
type DestroyResponse struct {
	OK bool
}

// DescribeResourcesRequest is the rpc wrapper for DescribeResources request
type DescribeResourcesRequest struct {
	Type string
	Tags map[string]string
}

// DescribeResourcesResponse is the rpc wrapper for the DescribeResources response
type DescribeResourcesResponse struct {
	Descriptions []resource.Description
}
