package plugin

import "github.com/docker/infrakit/pkg/spi"

// APIsRequest is the rpc wrapper for the APIs method args.
type APIsRequest struct {
}

// APIsResponse is the rpc wrapper for the APIs return value.
type APIsResponse struct {
	APIs []spi.APISpec
}
