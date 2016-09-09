package instance

import (
	"encoding/json"
	"github.com/docker/libmachete/spi/instance"
)

// TODO(chungers) -- move this back to libmachete or add to plugin-helper

// ProvisionRequest is the JSON format for calls to Provision.
type ProvisionRequest struct {
	Request *json.RawMessage
	Volume  *instance.VolumeID
	Tags    map[string]string
}
