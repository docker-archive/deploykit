package manager

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
)

// proxyForGroupPlugin registers a group plugin that this manager will proxy for.
func (m *manager) proxyForGroupPlugin(name string) (group.Plugin, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.backendName = name

	// A late-binding proxy so that we don't have a problem with having to
	// start up the manager as the last of all the plugins.
	return NewProxy(func() (group.Plugin, error) {
		endpoint, err := m.plugins.Find(name)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(endpoint.Address), nil
	}), nil
}

// This implements the Group Plugin interface to support single group-only operations
// This is contrast with the declarative semantics of commit.  It offers an imperative
// operation by operation interface to the user.

func (m *manager) updateConfig(spec group.Spec) error {
	log.Debugln("Updating config", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := GlobalSpec{}

	err := m.snapshot.Load(&stored)
	// TODO: More robust (type-based) error handling.
	if err != nil && err.Error() != "not-found" {
		return err
	}
	// if not-found ok to continue...

	if stored.Groups == nil {
		stored.Groups = map[group.ID]PluginSpec{}
	}

	buff, err := json.MarshalIndent(spec, "  ", "  ")
	if err != nil {
		return err
	}
	raw := json.RawMessage(buff)
	stored.Groups[spec.ID] = PluginSpec{
		Plugin:     m.backendName,
		Properties: &raw,
	}
	log.Debugln("Saving updated config", stored)

	return m.snapshot.Save(stored)
}

func (m *manager) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	err := make(chan error)
	m.backendOps <- backendOp{
		name: "watch",
		operation: func() error {
			log.Debugln("Proxy WatchGroup:", grp)
			if !pretend {
				if err := m.updateConfig(grp); err != nil {
					return err
				}
			}
			_, err := m.Plugin.CommitGroup(grp, pretend)
			return err
		},
		err: err,
	}
	return "TODO(chungers): Allow the commit details string to be plumbed through", <-err
}
