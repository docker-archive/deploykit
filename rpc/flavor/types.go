package flavor

import (
	"encoding/json"

	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/instance"
)

// ValidateRequest is the rpc wrapper for request parameters to Validate
type ValidateRequest struct {
	Properties json.RawMessage
	Allocation types.AllocationMethod
}

// ValidateResponse is the rpc wrapper for the results of Validate
type ValidateResponse struct {
	OK bool
}

// PrepareRequest is the rpc wrapper of the params to Prepare
type PrepareRequest struct {
	Properties json.RawMessage
	Spec       instance.Spec
	Allocation types.AllocationMethod
}

// PrepareResponse is the rpc wrapper of the result of Prepare
type PrepareResponse struct {
	Spec instance.Spec
}

// HealthyRequest is the rpc wrapper of the params to Healthy
type HealthyRequest struct {
	Properties json.RawMessage
	Instance   instance.Description
}

// HealthyResponse is the rpc wrapper of the result of Healthy
type HealthyResponse struct {
	Health flavor.Health
}

// DrainRequest is the rpc wrapper of the params to Drain
type DrainRequest struct {
	Properties json.RawMessage
	Instance   instance.Description
}

// DrainResponse is the rpc wrapper of the result of Drain
type DrainResponse struct {
	OK bool
}

// RPCService is the interface for exposing the plugin methods to rpc
type RPCService interface {

	// Validate is the rpc method for flavor Validate
	Validate(req *ValidateRequest, resp *ValidateResponse) error

	// Prepare is the rpc method for flavor Prepare
	Prepare(req *PrepareRequest, resp *PrepareResponse) error

	// Healthy is the rpc method for flavor Healthy
	Healthy(req *HealthyRequest, resp *HealthyResponse) error

	// Drain is the rpc method for flavor Drain
	// It's the inverse of prepare before provision and happens before destroy.
	Drain(req *DrainRequest, resp *DrainResponse) error
}
