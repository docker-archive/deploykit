package event

import (
	"github.com/docker/infrakit/pkg/spi/event"
)

// TopicsRequest is the rpc wrapper for request parameters to Topics
type TopicsRequest struct {
}

// TopicsResponse is the rpc wrapper for the results of Topics
type TopicsResponse struct {
	Topics []event.Topic
}
