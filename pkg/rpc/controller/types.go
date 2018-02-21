package controller

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

// ChangeRequest is the common request message for Plan and Commit
type ChangeRequest struct {
	Name      plugin.Name
	Operation controller.Operation
	Spec      types.Spec
}

// Plugin implements pkg/rpc/internal/Addressable
func (r ChangeRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
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

// Plugin implements pkg/rpc/internal/Addressable
func (r FindRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// FindResponse is the common response message for Describe and Free
type FindResponse struct {
	Name    plugin.Name
	Objects []types.Object
}
