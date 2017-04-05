package gcloud

import (
	"cloud.google.com/go/compute/metadata"
)

// APIMetadata gives access to the GCE metadata service.
type APIMetadata interface {
	// GetHostname returns current instance's fully qualified hostname.
	GetHostname() (string, error)
}

type metadataWrapper struct{}

// NewAPIMetadata creates a new APIMetadata instance.
func NewAPIMetadata() APIMetadata {
	return &metadataWrapper{}
}

func (m *metadataWrapper) GetHostname() (string, error) {
	return metadata.Hostname()
}
