package metadata

import (
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/types"
)

var (
	// InterfaceSpec is the current name and version of the Metadata API.
	InterfaceSpec = spi.InterfaceSpec{
		Name:    "Metadata",
		Version: "0.1.0",
	}

	// UpdatableInterfaceSpec is the current name and version of the Metadata API.
	UpdatableInterfaceSpec = spi.InterfaceSpec{
		Name:    "Updatable",
		Version: "0.1.0",
	}
)

// Plugin is the interface for metadata-related operations.
type Plugin interface {

	// List returns a list of *child nodes* given a path, which is specified as a slice
	List(path types.Path) (child []string, err error)

	// Get retrieves the value at path given.
	Get(path types.Path) (value *types.Any, err error)
}

// Change is an update to the metadata / config
type Change struct {
	Path  types.Path
	Value *types.Any
}

// Updatable is the interface for updating metadata
type Updatable interface {

	// Plugin - embeds a readonly plugin interface
	Plugin

	// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
	Changes(changes []Change) (original, proposed *types.Any, cas string, err error)

	// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
	// optimistic concurrency control.
	Commit(proposed *types.Any, cas string) error
}
