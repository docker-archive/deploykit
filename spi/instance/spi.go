package instance

// A Provisioner is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Provisioner interface {
	// Provision creates a new instance.
	Provision(request string, volume *VolumeID) (*ID, error)

	// Destroy terminates an existing instance.
	Destroy(instance ID) error

	// DescribeInstances returns descriptions of all instances included in a group.
	DescribeInstances(group GroupID) ([]Description, error)

	// ShellExec executes a shell command on an instance, and returns the combined (stderr and stdout) output.
	ShellExec(id ID, shellCode string) (*string, error)
}
