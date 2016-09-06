package instance

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
