package util

import (
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/spi/instance"
)

type provisionRequest struct {
	Group instance.GroupID `json:"group"`
}

// GroupFromRequest extracts the group ID from an otherwise opaque provisioning request.
func GroupFromRequest(request string) (*instance.GroupID, error) {
	req := provisionRequest{}
	err := json.Unmarshal([]byte(request), &req)
	if err != nil {
		return nil, err
	}

	if req.Group == "" {
		return nil, errors.New("Group must not be empty")
	}

	return &req.Group, nil
}
