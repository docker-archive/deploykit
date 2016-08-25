package util

import (
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/spi/instance"
)

type provisionRequest struct {
	Group instance.GroupID `json:"group"`
	Count uint             `json:"count"`
}

// GroupFromRequest extracts the group ID and count from an otherwise opaque provisioning request.
func GroupAndCountFromRequest(request string) (*instance.GroupID, uint, error) {
	req := provisionRequest{}
	err := json.Unmarshal([]byte(request), &req)
	if err != nil {
		return nil, 0, err
	}

	if req.Group == "" {
		return nil, 0, errors.New("Group must not be empty")
	}

	return &req.Group, req.Count, nil
}
