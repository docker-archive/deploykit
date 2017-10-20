package metadata

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/imdario/mergo"
)

var (
	log    = logutil.New("module", "plugin/metadata/updatable")
	debugV = logutil.V(300)
)

// LoadFunc is the function for returning the original to modify
type LoadFunc func() (original *types.Any, err error)

// CommitFunc is the function for handling commit
type CommitFunc func(proposed *types.Any) error

// NewUpdatablePlugin assembles the implementations into a Updatable implementation
func NewUpdatablePlugin(reader metadata.Plugin, commit CommitFunc) metadata.Updatable {
	return &updatable{
		Plugin: reader,
		commit: commit,
	}
}

type updatable struct {
	metadata.Plugin
	commit CommitFunc
}

// changeSet returns a sparse map where the kv pairs of path / value have been
// apply to a nested map structure.
func changeSet(changes []metadata.Change) (*types.Any, error) {
	changed := map[string]interface{}{}
	for _, c := range changes {
		if !types.Put(c.Path, c.Value, &changed) {
			return nil, fmt.Errorf("can't apply change %s %s", c.Path, c.Value)
		}
	}
	return types.AnyValue(changed)
}

func (p updatable) load() (original *types.Any, err error) {
	return p.Plugin.Get(types.Dot)
}

// Changes sends a batch of changes and gets in return a proposed view of configuration and a cas hash.
func (p updatable) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {

	if u, is := p.Plugin.(metadata.Updatable); is {
		return u.Changes(changes)
	}

	// first read the data to be modified
	original, err = p.load()
	if err != nil {
		return
	}
	log.Debug("original", "original", original.String(), "V", debugV)

	var current map[string]interface{}
	if err = original.Decode(&current); err != nil {
		return
	}
	log.Debug("decoded", "current", current, "V", debugV)

	changeSet, e := changeSet(changes)
	if e != nil {
		err = e
		return
	}
	log.Debug("changeset", "changeset", changeSet, "V", debugV)

	var applied map[string]interface{}
	if err = changeSet.Decode(&applied); err != nil {
		return
	}

	if err = mergo.Merge(&applied, &current); err != nil {
		return
	}

	log.Debug("decoded2", "applied", applied, "V", debugV)

	// encoded it back to bytes
	proposed, err = types.AnyValue(applied)
	if err != nil {
		return
	}

	log.Debug("proposed", "proposed", proposed.String(), "V", debugV)

	cas = types.Fingerprint(original, proposed)
	return
}

// Commit asks the plugin to commit the proposed view with the cas.  The cas is used for
// optimistic concurrency control.
func (p updatable) Commit(proposed *types.Any, cas string) error {

	log.Debug("commit", "proposed", proposed, "cas", cas, "V", debugV)

	u, is := p.Plugin.(metadata.Updatable)

	if is {

		log.Debug("forward commit to backend", "plugin", u, "V", debugV)

		if err := u.Commit(proposed, cas); err != nil {
			return err
		}

	} else {

		log.Debug("commit in local layer", "V", debugV)

		// first read the data to be modified
		buff, err := p.load()
		if err != nil {
			return err
		}

		hash := types.Fingerprint(buff, proposed)
		if hash != cas {
			return fmt.Errorf("cas mismatch")
		}
	}

	return p.commit(proposed)
}
