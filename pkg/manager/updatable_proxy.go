package manager

import (
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (m *manager) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	return m.Updatable.Changes(changes)
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (m *manager) Commit(proposed *types.Any, cas string) error {
	return m.Updatable.Commit(proposed, cas)
}
