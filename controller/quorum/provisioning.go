package quorum

import (
	"encoding/json"
	"errors"
	"github.com/docker/libmachete/spi/group"
)

type provisionRequest struct {
	Group group.ID `json:"group"`
	Count uint     `json:"count"`
}

// groupAndCountFromRequest extracts the group ID and count from an otherwise opaque provisioning request.
func groupAndCountFromRequest(request string) (*group.ID, uint, error) {
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
