package manager

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/plugin"
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
		endpoint, err := m.plugins.Find(plugin.Name(name))
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(endpoint.Address)
	}), nil
}

func (m *manager) updateConfig(spec group.Spec) error {
	log.Debugln("Updating config", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := GlobalSpec{}

	err := m.snapshot.Load(&stored)
	if err != nil {
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

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "commit",
		operation: func() error {
			log.Infoln("Proxy CommitGroup:", grp)
			if !pretend {
				if err := m.updateConfig(grp); err != nil {
					log.Warningln("Error updating", err)
					return err
				}
			}
			resp, cerr := m.Plugin.CommitGroup(grp, pretend)
			log.Infoln("Responses from CommitGroup:", resp, cerr)
			resultChan <- []interface{}{resp, cerr}
			return err
		},
	}

	r := <-resultChan

	if v, has := r[0].(string); has {
		resp = v
	}
	if v, has := r[1].(error); has && v != nil {
		err = v
	}
	return
}
