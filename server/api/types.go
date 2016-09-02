package api

import (
	"encoding/json"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
)

// ProvisionRequest is the JSON format for calls to Provision.
type ProvisionRequest struct {
	Group   group.ID
	Request *json.RawMessage
	Volume  *instance.VolumeID
}
