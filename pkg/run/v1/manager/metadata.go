package manager

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/store"
	"github.com/docker/infrakit/pkg/types"
)

type metadataModel struct {
	snapshot store.Snapshot
	manager  manager.Manager
}

func (updatable *metadataModel) commit(proposed *types.Any) error {
	newState := struct {
		Groups map[group.ID]plugin.Spec
	}{}

	if err := proposed.Decode(&newState); err != nil {
		return err
	}
	// Hacky --- there's a mismatch with how the Commit's schema and the internal
	// store's schema --> we made the map based internal representation updatable
	// so that it's possbile to use paths that contain object names (e.g. Groups/cattle/Properties as
	// opposed to Groups/0/Properties).  So here we'd have to transform the object to
	// make it the right shape.
	// leaving this code here because we will have a new schema and this will be replaced soon.
	groups := []group.Spec{}
	for _, plugin := range newState.Groups {

		spec := group.Spec{}
		if err := plugin.Properties.Decode(&spec); err != nil {
			return err
		}

		groups = append(groups, spec)
	}

	groupImpl, ok := updatable.manager.(group.Plugin)
	if !ok {
		return fmt.Errorf("manager does not implement group.Plugin interface")
	}

	for _, spec := range groups {
		if _, err := groupImpl.CommitGroup(spec, false); err != nil {
			return err
		}
	}
	return nil
}

func (updatable *metadataModel) load() (original *types.Any, err error) {
	var state interface{}
	if err := updatable.snapshot.Load(&state); err != nil {
		return nil, err
	}
	return types.AnyValue(state)
}

func (updatable *metadataModel) pluginModel() (chan func(map[string]interface{}), chan struct{}) {
	// Start a poller to load the snapshot and make that available as metadata
	model := make(chan func(map[string]interface{}))
	stop := make(chan struct{})
	go func() {
		tick := time.Tick(1 * time.Second)
		for {
			select {
			case <-tick:
				snapshot := map[string]interface{}{}

				// update leadership
				if isLeader, err := updatable.manager.IsLeader(); err == nil {
					model <- func(view map[string]interface{}) {
						types.Put([]string{"leader"}, isLeader, view)
					}
				} else {
					log.Warn("Cannot check leader for metadata", "err", err)
				}

				// update config
				if err := updatable.snapshot.Load(&snapshot); err == nil {
					model <- func(view map[string]interface{}) {
						types.Put([]string{"configs"}, snapshot, view)
					}
				} else {
					log.Warn("Cannot load snapshot for metadata", "err", err)
				}

			case <-stop:
				log.Info("Snapshot updater stopped")
				return
			}
		}
	}()
	return model, stop
}
