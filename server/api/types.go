package api

import (
	"encoding/json"
	"github.com/docker/libmachete/spi/instance"
)

// ProvisionRequest is the JSON format for calls to Provision.
type ProvisionRequest struct {
	Request *json.RawMessage
	Volume  *instance.VolumeID
	Tags    map[string]string
}
