package manager

import (
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Metadata returns a map of metadata plugins
func (m *manager) Metadata() (map[string]metadata.Plugin, error) {
	plugins := map[string]metadata.Plugin{
		//		".":                         m,
		"status":                    m.Status,
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

func initStatusMetadata(m *manager) metadata.Plugin {
	updates, stop := m.status()
	m.doneStatusUpdates = stop
	return metadata_plugin.NewPluginFromChannel(updates)
}

func (m *manager) status() (chan func(map[string]interface{}), chan struct{}) {
	// Start a poller to load the snapshot and make that available as metadata
	model := make(chan func(map[string]interface{}))
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-m.refreshStatus:

				// update leadership
				if isLeader, err := m.IsLeader(); err == nil {
					model <- func(view map[string]interface{}) {
						types.Put([]string{"leader"}, isLeader, view)
					}
				} else {
					log.Warn("Cannot check leader for metadata", "err", err)
				}

				// update config
				snapshot := map[string]interface{}{}
				objects, err := m.Inspect()
				if err != nil {
					log.Warn("Error inspecting manager states", "err", err)
					continue
				}
				for _, o := range objects {
					snapshot[o.Spec.Metadata.Name] = o.Spec
				}
				model <- func(view map[string]interface{}) {
					types.Put([]string{"configs"}, snapshot, view)
				}

			case <-stop:
				log.Info("Snapshot updater stopped")
				return
			}
		}
	}()
	return model, stop
}
