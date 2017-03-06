package event

import (
	"github.com/docker/infrakit/pkg/types"
)

// ListRequest is the rpc wrapper for request parameters to List
type ListRequest struct {
	Topic types.Path
}

// ListResponse is the rpc wrapper for the results of List
type ListResponse struct {
	Nodes []string
}
