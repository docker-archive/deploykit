package metadata

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

var log = logutil.New("module", "plugin/metadata/updatable")

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
func (p updatable) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	// first read the data to be modified
	original, err = p.load()
	if err != nil {
		return
	}
	log.Info("original", "original", original.String())

	var current map[string]interface{}
	if err = original.Decode(&current); err != nil {
		return
	}
	log.Info("decoded", "current", current)

	changeSet, e := changeSet(changes)
	if e != nil {
		err = e
		return
	}
	log.Info("changeset", "changeset", changeSet)

	var applied map[string]interface{}
	if err = changeSet.Decode(&applied); err != nil {
		return
	}

	if err = mergo.Merge(&applied, &current); err != nil {
		return
	}

	log.Info("decoded2", "applied", applied)

	// encoded it back to bytes
	proposed, err = types.AnyValue(applied)
	if err != nil {
		return
	}

	log.Info("proposed", "proposed", proposed.String())

	cas = types.Fingerprint(original, proposed)
	return
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (p updatable) Commit(proposed *types.Any, cas string) error {

	// first read the data to be modified
	buff, err := p.load()
	if err != nil {
		return err
	}

	hash := types.Fingerprint(buff, proposed)
	if hash != cas {
		return fmt.Errorf("cas mismatch")
	}

	return p.commit(proposed)
}
