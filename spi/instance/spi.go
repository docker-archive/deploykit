package instance

// Plugin is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Plugin interface {
	// Provision creates a new instance.
	Provision(req string, volume *VolumeID, tags map[string]string) (*ID, error)

	// Destroy terminates an existing instance.
	Destroy(instance ID) error

	// DescribeInstances returns descriptions of all instances matching all of the provided tags.
	DescribeInstances(tags map[string]string) ([]Description, error)
}
