package manager

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

func (m *manager) updateConfig(spec group.Spec) error {
	log.Debug("Updating config", "spec", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}
	err := stored.store(m.snapshot)
	if err != nil {
		return err
	}

	stored.updateGroupSpec(spec, plugin.Name(m.backendName))
	log.Debug("Saving updated config", "config", stored)

	return stored.store(m.snapshot)
}

func (m *manager) removeConfig(id group.ID) error {
	log.Debug("Removing config", "groupID", id)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}
	err := stored.load(m.snapshot)
	if err != nil {
		return err
	}

	stored.removeGroup(id)
	log.Debug("Saving updated config", "config", stored)

	return stored.store(m.snapshot)
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {

	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "commit",
		operation: func() error {
			log.Debug("Manager CommitGroup", "spec", grp, "V", debugV)

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

// Serialized describe group
func (m *manager) DescribeGroup(id group.ID) (desc group.Description, err error) {
	log.Debug("Describe group", "id", id, "V", debugV)
	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "describe",
		operation: func() error {
			log.Debug("Manager DescribeGroup", "id", id, "V", debugV)

			var txnResp group.Description
			var txnErr error

			// Always send a response so we don't block forever
			defer func() {
				resultChan <- []interface{}{txnResp, txnErr}
			}()

			txnResp, txnErr = m.Plugin.DescribeGroup(id)
			return txnErr
		},
	}

	r := <-resultChan
	if v, has := r[0].(group.Description); has {
		desc = v
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
			log.Debug("Manager DestroyGroup", "groupID", id, "V", debugV)

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
			log.Debug("Manager FreeGroup", "groupID", id, "V", debugV)

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
	log.Debug("manager.DestroyInstances", "id", id, "instances", instances, "V", debugV)
	resultChan := make(chan []interface{})

	m.backendOps <- backendOp{
		name: "destroyInstances",
		operation: func() error {
			log.Debug("Manager DestroyInstances", "groupID", id, "instances", instances, "V", debugV)

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

func (m *manager) loadGroupSpec(id group.ID) (group.Spec, error) {
	// load the config
	config := globalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := config.load(m.snapshot)
	if err != nil {
		log.Warn("Error loading config", "err", err)
		return group.Spec{}, err
	}
	return config.getGroupSpec(id)
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
