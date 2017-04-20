package metadata

import (
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements metadata.Plugin
type Plugin struct {

	// DoList implements List via function
	DoList func(path types.Path) (child []string, err error)

	// DoGet implements Get via function
	DoGet func(path types.Path) (value *types.Any, err error)
}

// List lists the child nodes under path
func (t *Plugin) List(path types.Path) (child []string, err error) {
	return t.DoList(path)
}

// Get gets the value
func (t *Plugin) Get(path types.Path) (value *types.Any, err error) {
	return t.DoGet(path)
}

// Updatable implements metadata.Updatable
type Updatable struct {
	Plugin

	// DoChanges sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
	DoChanges func(changes []metadata.Change) (proposed *types.Any, cas string, err error)

	// DoCommit asks the plugin to commit the proposed view with the cas.  The cas is used for
	// optimistic concurrency control.
	DoCommit func(proposed *types.Any, cas string) error
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (u *Updatable) Changes(changes []metadata.Change) (proposed *types.Any, cas string, err error) {
	return u.DoChanges(changes)
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for mvcc
func (u *Updatable) Commit(proposed *types.Any, cas string) error {
	return u.DoCommit(proposed, cas)
}
