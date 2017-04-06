package application

import (
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/types"
)

// ValidateRequest is the rpc wrapper for request parameters to Validate
type ValidateRequest struct {
	Type       string
	Properties *types.Any
}

// ValidateResponse is the rpc wrapper for the results of Validate
type ValidateResponse struct {
	Type string
	OK   bool
}

// HealthyRequest is the rpc wrapper of the params to Healthy
type HealthyRequest struct {
	Type       string
	Properties *types.Any
}

// HealthyResponse is the rpc wrapper of the result of Healthy
type HealthyResponse struct {
	Type   string
	Health application.Health
}

// UpdateRequest is the rpc wrapper of the params to Update
type UpdateRequest struct {
	Type    string
	Message *application.Message
}

// UpdateResponse is the rpc wrapper of the result of Update
type UpdateResponse struct {
	Type string
	OK   bool
}
