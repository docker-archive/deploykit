package controller

import (
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/types"
)

// ChangeRequest is the common request message for Plan and Commit
type ChangeRequest struct {
	Name      plugin.Name
	Operation controller.Operation
	Spec      types.Spec
}

// ChangeResponse is the common response message for Plan and Commit
type ChangeResponse struct {
	Name   plugin.Name
	Object types.Object
	Plan   controller.Plan
}

// FindRequest is the common request message for Describe and Free
type FindRequest struct {
	Name     plugin.Name
	Metadata *types.Metadata
}

// FindResponse is the common response message for Describe and Free
type FindResponse struct {
	Name    plugin.Name
	Objects []types.Object
}
