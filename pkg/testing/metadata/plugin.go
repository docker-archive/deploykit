package metadata

import (
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements metadata.Plugin
type Plugin struct {

	// DoKeys implements Keys via function
	DoKeys func(path types.Path) (child []string, err error)

	// DoGet implements Get via function
	DoGet func(path types.Path) (value *types.Any, err error)
}

// Keys lists the child nodes under path
func (t *Plugin) Keys(path types.Path) (child []string, err error) {
	return t.DoKeys(path)
}

// Get gets the value
func (t *Plugin) Get(path types.Path) (value *types.Any, err error) {
	return t.DoGet(path)
}

// Updatable implements metadata.Updatable
type Updatable struct {
	Plugin

	// DoChanges sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
	DoChanges func(changes []metadata.Change) (original, proposed *types.Any, cas string, err error)

	// DoCommit asks the plugin to commit the proposed view with the cas.  The cas is used for
	// optimistic concurrency control.
	DoCommit func(proposed *types.Any, cas string) error
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (u *Updatable) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	return u.DoChanges(changes)
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for mvcc
func (u *Updatable) Commit(proposed *types.Any, cas string) error {
	return u.DoCommit(proposed, cas)
}
