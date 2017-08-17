package manager

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/types"
)

// proxyForGroupPlugin registers a group plugin that this manager will proxy for.
func (m *manager) proxyForGroupPlugin(name string) (group.Plugin, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.backendName = name

	// A late-binding proxy so that we don't have a problem with having to
	// start up the manager as the last of all the plugins.
	return newProxy(func() (group.Plugin, error) {
		endpoint, err := m.plugins.Find(plugin.Name(name))
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(endpoint.Address)
	}), nil
}

func (m *manager) updateConfig(spec group.Spec) error {
	log.Debug("Updating config", "spec", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}

	err := m.snapshot.Load(&stored)
	if err != nil {
		return err
	}

	// if not-found ok to continue...

	if stored.Groups == nil {
		stored.Groups = map[group.ID]plugin.Spec{}
	}

	any, err := types.AnyValue(spec)
	if err != nil {
		return err
	}
	stored.Groups[spec.ID] = plugin.Spec{
		Plugin:     plugin.Name(m.backendName),
		Properties: any,
	}
	log.Debug("Saving updated config", "config", stored)

	return m.snapshot.Save(stored)
}

func (m *manager) removeConfig(id group.ID) error {
	log.Debug("Removing config", "groupID", id)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}

	err := m.snapshot.Load(&stored)
	if err != nil {
		return err
	}

	// if not-found just return without error.
	if stored.Groups == nil {
		return nil
	}

	delete(stored.Groups, id)
	log.Debug("Saving updated config", "config", stored)

	return m.snapshot.Save(stored)
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "commit",
		operation: func() error {
			log.Info("Proxy CommitGroup", "spec", grp)

			var txnResp string
			var txnErr error

			// Always send a response so we don't block forever
			defer func() {
				resultChan <- []interface{}{txnResp, txnErr}
			}()

			// We first update the user's desired state first
			if !pretend {
				if updateErr := m.updateConfig(grp); updateErr != nil {
					log.Warn("Error updating", "err", updateErr)
					txnErr = updateErr
					txnResp = "Cannot update spec. Abort"
					return txnErr
				}
			}

			txnResp, txnErr = m.Plugin.CommitGroup(grp, pretend)
			return txnErr
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

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) DestroyGroup(id group.ID) (err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "destroy",
		operation: func() error {

			log.Info("Proxy DestroyGroup", "groupID", id)

			var txnErr error

			// Always send a response so we don't block forever
			defer func() {
				resultChan <- []interface{}{txnErr}
			}()

			// We first update the user's desired state first
			if removeErr := m.removeConfig(id); removeErr != nil {
				log.Warn("Error updating/ remove", "err", removeErr)
				txnErr = removeErr
				return txnErr
			}

			txnErr = m.Plugin.DestroyGroup(id)
			return txnErr
		},
	}

	r := <-resultChan
	if v, has := r[0].(error); has && v != nil {
		err = v
	}
	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) FreeGroup(id group.ID) (err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "free",
		operation: func() error {

			log.Info("Proxy FreeGroup", "groupID", id)

			var txnErr error

			// Always send a response so we don't block forever
			defer func() {
				resultChan <- []interface{}{txnErr}
			}()

			// We first update the user's desired state first
			if removeErr := m.removeConfig(id); removeErr != nil {
				log.Warn("Error updating / remove", "err", removeErr)
				txnErr = removeErr
				return txnErr
			}

			txnErr = m.Plugin.FreeGroup(id)
			return txnErr
		},
	}

	r := <-resultChan
	if v, has := r[0].(error); has && v != nil {
		err = v
	}
	return
}
