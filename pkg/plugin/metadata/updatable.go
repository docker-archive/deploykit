package metadata

import (
	"crypto/md5"
	"fmt"

	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// LoadFunc is the function for returning the original to modify
type LoadFunc func() (original *types.Any, err error)

// CommitFunc is the function for handling commit
type CommitFunc func(proposed *types.Any) error

// NewUpdatablePlugin assembles the implementations into a Updatable implementation
func NewUpdatablePlugin(reader metadata.Plugin, load LoadFunc, commit CommitFunc) metadata.Updatable {
	return &updatable{
		Plugin: reader,
		load:   load,
		commit: commit,
	}
}

type updatable struct {
	metadata.Plugin
	load   LoadFunc
	commit CommitFunc
}

func hash(m ...*types.Any) string {
	h := md5.New()
	for _, mm := range m {
		h.Write(mm.Bytes())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// changeSet returns a sparse map where the kv pairs of path / value have been
// apply to a nested map structure.
func changeSet(changes []metadata.Change) (*types.Any, error) {
	changed := map[string]interface{}{}
	for _, c := range changes {
		if !types.Put(c.Path, c.Value, changed) {
			return nil, fmt.Errorf("can't apply change %s %s", c.Path, c.Value)
		}
	}
	return types.AnyValue(changed)
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (p updatable) Changes(changes []metadata.Change) (proposed *types.Any, cas string, err error) {

	// first read the data to be modified
	buff, err := p.load()
	if err != nil {
		return nil, "", err
	}

	var original interface{}
	if err := buff.Decode(&original); err != nil {
		return nil, "", err
	}

	changeSet, err := changeSet(changes)
	if err != nil {
		return nil, "", err
	}

	// apply the changes using the originalVal as default
	if err := changeSet.Decode(&original); err != nil {
		return nil, "", err
	}

	// encoded it back to bytes
	applied, err := types.AnyValue(original)
	if err != nil {
		return nil, "", err
	}

	hash := hash(buff, applied)
	return applied, hash, nil
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (p updatable) Commit(proposed *types.Any, cas string) error {

	// first read the data to be modified
	buff, err := p.load()
	if err != nil {
		return err
	}

	hash := hash(buff, proposed)
	if hash != cas {
		return fmt.Errorf("cas mismatch")
	}

	return p.commit(proposed)
}
