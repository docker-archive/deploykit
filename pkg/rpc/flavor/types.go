package flavor

import (
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// ValidateRequest is the rpc wrapper for request parameters to Validate
type ValidateRequest struct {
	Type       string
	Properties *types.Any
	Allocation group.AllocationMethod
}

// ValidateResponse is the rpc wrapper for the results of Validate
type ValidateResponse struct {
	Type string
	OK   bool
}

// PrepareRequest is the rpc wrapper of the params to Prepare
type PrepareRequest struct {
	Type       string
	Properties *types.Any
	Spec       instance.Spec
	Allocation group.AllocationMethod
	Index      group.Index
}

// PrepareResponse is the rpc wrapper of the result of Prepare
type PrepareResponse struct {
	Type string
	Spec instance.Spec
}

// HealthyRequest is the rpc wrapper of the params to Healthy
type HealthyRequest struct {
	Type       string
	Properties *types.Any
	Instance   instance.Description
}

// HealthyResponse is the rpc wrapper of the result of Healthy
type HealthyResponse struct {
	Type   string
	Health flavor.Health
}

// DrainRequest is the rpc wrapper of the params to Drain
type DrainRequest struct {
	Type       string
	Properties *types.Any
	Instance   instance.Description
}

// DrainResponse is the rpc wrapper of the result of Drain
type DrainResponse struct {
	Type string
	OK   bool
}
