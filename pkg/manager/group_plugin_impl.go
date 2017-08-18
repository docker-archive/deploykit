package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
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

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) DestroyInstances(id group.ID, instances []instance.ID) (err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "destroyInstances",
		operation: func() error {

			log.Info("Proxy DestroyInstances", "groupID", id, "instances", instances)

			var txnErr error

			// Always send a response so we don't block forever
			defer func() {
				resultChan <- []interface{}{txnErr}
			}()

			txnErr = m.Plugin.DestroyInstances(id, instances)
			return txnErr
		},
	}

	r := <-resultChan
	if v, has := r[0].(error); has && v != nil {
		err = v
	}
	return
}

func (m *manager) loadGroupSpec(id group.ID) (found group.Spec, err error) {
	// load the config
	config := globalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err = m.snapshot.Load(&config)
	if err != nil {
		log.Warn("Error loading config", "err", err)
		return
	}
	for gid, g := range config.Groups {
		if gid == id {
			spec := group.Spec{}
			err = g.Properties.Decode(&spec)
			if err != nil {
				return
			}
			return spec, nil
		}
	}
	err = fmt.Errorf("group %v not found", id)
	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) SetSize(id group.ID, size int) error {
	spec, err := m.loadGroupSpec(id)
	if err != nil {
		return err
	}
	parsed, err := group_types.ParseProperties(spec)
	if err != nil {
		return err
	}
	if s := len(parsed.Allocation.LogicalIDs); s > 0 {
		return fmt.Errorf("cannot set size when logical ids are explicitly set")
	}
	parsed.Allocation.Size = uint(size)
	spec.Properties = types.AnyValueMust(parsed)
	_, err = m.CommitGroup(spec, false)
	return err
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) Size(id group.ID) (size int, err error) {
	spec, err := m.loadGroupSpec(id)
	if err != nil {
		return 0, err
	}
	parsed, err := group_types.ParseProperties(spec)
	if err != nil {
		return 0, err
	}
	if s := len(parsed.Allocation.LogicalIDs); s > 0 {
		size = s
		return size, nil
	}
	return int(parsed.Allocation.Size), nil
}
