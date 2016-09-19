package instance

import (
	"encoding/json"
)

// ID is the identifier for an instance.
type ID string

// VolumeID is the identifier for a storage volume.
type VolumeID string

// Description contains details about an instance.
type Description struct {
	ID               ID
	PrivateIPAddress string
	Tags             map[string]string
}

// Spec is a specification of an instance to be provisioned
type Spec struct {
	Properties       json.RawMessage
	Tags             map[string]string
	InitScript       string
	PrivateIPAddress *string
	Volume           *VolumeID
}
