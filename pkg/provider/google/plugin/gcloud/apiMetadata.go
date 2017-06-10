package gcloud

import (
	"cloud.google.com/go/compute/metadata"
)

// APIMetadata gives access to the GCE metadata service.
type APIMetadata interface {
	// Get returns a value from the metadata service.
	// The suffix is appended to "http://${GCE_METADATA_HOST}/computeMetadata/v1/".
	//
	// If the GCE_METADATA_HOST environment variable is not defined, a default of
	// 169.254.169.254 will be used instead.
	//
	// If the requested metadata is not defined, the returned error will
	// be of type NotDefinedError.
	Get(suffix string) (string, error)

	// ProjectID returns the current instance's project ID string.
	ProjectID() (string, error)

	// NumericProjectID returns the current instance's numeric project ID.
	NumericProjectID() (string, error)

	// InternalIP returns the instance's primary internal IP address.
	InternalIP() (string, error)

	// ExternalIP returns the instance's primary external (public) IP address.
	ExternalIP() (string, error)

	// Hostname returns the instance's hostname. This will be of the form
	// "<instanceID>.c.<projID>.internal".
	Hostname() (string, error)

	// InstanceTags returns the list of user-defined instance tags,
	// assigned when initially creating a GCE instance.
	InstanceTags() ([]string, error)

	// InstanceID returns the current VM's numeric instance ID.
	InstanceID() (string, error)

	// InstanceName returns the current VM's instance ID string.
	InstanceName() (string, error)

	// Zone returns the current VM's zone, such as "us-central1-b".
	Zone() (string, error)

	// InstanceAttributes returns the list of user-defined attributes,
	// assigned when initially creating a GCE VM instance. The value of an
	// attribute can be obtained with InstanceAttributeValue.
	InstanceAttributes() ([]string, error)

	// ProjectAttributes returns the list of user-defined attributes
	// applying to the project as a whole, not just this VM.  The value of
	// an attribute can be obtained with ProjectAttributeValue.
	ProjectAttributes() ([]string, error)

	// InstanceAttributeValue returns the value of the provided VM
	// instance attribute.
	//
	// If the requested attribute is not defined, the returned error will
	// be of type NotDefinedError.
	//
	// InstanceAttributeValue may return ("", nil) if the attribute was
	// defined to be the empty string.
	InstanceAttributeValue(attr string) (string, error)

	// ProjectAttributeValue returns the value of the provided
	// project attribute.
	//
	// If the requested attribute is not defined, the returned error will
	// be of type NotDefinedError.
	//
	// ProjectAttributeValue may return ("", nil) if the attribute was
	// defined to be the empty string.
	ProjectAttributeValue(attr string) (string, error)

	// Scopes returns the service account scopes for the given account.
	// The account may be empty or the string "default" to use the instance's
	// main account.
	Scopes(serviceAccount string) ([]string, error)
}

type metadataWrapper struct{}

// NewAPIMetadata creates a new APIMetadata instance.
func NewAPIMetadata() APIMetadata {
	return &metadataWrapper{}
}

func (m *metadataWrapper) Get(suffix string) (string, error) {
	return metadata.Get(suffix)
}

func (m *metadataWrapper) ProjectID() (string, error) {
	return metadata.ProjectID()
}

func (m *metadataWrapper) NumericProjectID() (string, error) {
	return metadata.NumericProjectID()
}

func (m *metadataWrapper) InternalIP() (string, error) {
	return metadata.InternalIP()
}

func (m *metadataWrapper) ExternalIP() (string, error) {
	return metadata.ExternalIP()
}

func (m *metadataWrapper) Hostname() (string, error) {
	return metadata.Hostname()
}

func (m *metadataWrapper) InstanceTags() ([]string, error) {
	return metadata.InstanceTags()
}

func (m *metadataWrapper) InstanceID() (string, error) {
	return metadata.InstanceID()
}

func (m *metadataWrapper) InstanceName() (string, error) {
	return metadata.InstanceName()
}

func (m *metadataWrapper) Zone() (string, error) {
	return metadata.Zone()
}

func (m *metadataWrapper) InstanceAttributes() ([]string, error) {
	return metadata.InstanceAttributes()
}

func (m *metadataWrapper) ProjectAttributes() ([]string, error) {
	return metadata.ProjectAttributes()
}

func (m *metadataWrapper) InstanceAttributeValue(attr string) (string, error) {
	return metadata.InstanceAttributeValue(attr)
}

func (m *metadataWrapper) ProjectAttributeValue(attr string) (string, error) {
	return metadata.ProjectAttributeValue(attr)
}

func (m *metadataWrapper) Scopes(serviceAccount string) ([]string, error) {
	return metadata.Scopes(serviceAccount)
}
