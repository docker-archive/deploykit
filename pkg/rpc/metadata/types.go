package metadata

import (
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// ListRequest is the rpc wrapper for request parameters to List
type ListRequest struct {
	Name plugin.Name
	Path types.Path
}

// Plugin implements pkg/rpc/internal/Addressable
func (r ListRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// ListResponse is the rpc wrapper for the results of List
type ListResponse struct {
	Name  plugin.Name
	Nodes []string
}

// GetRequest is the rpc wrapper of the params to Get
type GetRequest struct {
	Name plugin.Name
	Path types.Path
}

// Plugin implements pkg/rpc/internal/Addressable
func (r GetRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// GetResponse is the rpc wrapper of the result of Get
type GetResponse struct {
	Name  plugin.Name
	Value *types.Any
}

// ChangesRequest is the rpc wrapper of the params to Changes
type ChangesRequest struct {
	Name    plugin.Name
	Changes []metadata.Change
}

// Plugin implements pkg/rpc/internal/Addressable
func (r ChangesRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// ChangesResponse is the rpc wrapper of the params to Changes
type ChangesResponse struct {
	Name     plugin.Name
	Original *types.Any
	Proposed *types.Any
	Cas      string
}

// CommitRequest is the rpc wrapper of the params to Commit
type CommitRequest struct {
	Name     plugin.Name
	Proposed *types.Any
	Cas      string
}

// Plugin implements pkg/rpc/internal/Addressable
func (r CommitRequest) Plugin() (plugin.Name, error) {
	return r.Name, nil
}

// CommitResponse is the rpc wrapper of the params to Commit
type CommitResponse struct {
	Name plugin.Name
}
