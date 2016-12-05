package plugin

import "github.com/docker/infrakit/pkg/spi"

// ImplementsRequest is the rpc wrapper for the Implements method args.
type ImplementsRequest struct {
}

// ImplementsResponse is the rpc wrapper for the Implements return value.
type ImplementsResponse struct {
	APIs []spi.InterfaceSpec
}
