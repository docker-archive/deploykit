package manager

import (
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Metadata returns a map of metadata plugins
func (m *manager) Metadata() (map[string]metadata.Updatable, error) {
	plugins := map[string]metadata.Updatable{
		".": m,
		m.Options.Metadata.Lookup(): m,
	}
	return plugins, nil
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (m *manager) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	return m.Updatable.Changes(changes)
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (m *manager) Commit(proposed *types.Any, cas string) error {
	return m.Updatable.Commit(proposed, cas)
}
