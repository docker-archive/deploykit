package instance

import "github.com/docker/libmachete/machete/spi"

// A Provisioner is a vendor-agnostic API used to create and manage resources with an infrastructure provider.
type Provisioner interface {
	// Provision creates a new instance.
	Provision(request string) (*ID, *spi.Error)

	// Destroy terminates an existing instance.
	Destroy(instance ID) *spi.Error

	// ListGroup returns all instances included in a group.
	ListGroup(group GroupID) ([]ID, *spi.Error)
}
