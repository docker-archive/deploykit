package manager

import (
	"fmt"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Groups returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Groups() (map[group.ID]group.Plugin, error) {
	groups := map[group.ID]group.Plugin{
		group.ID("groups"): m,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return groups, nil
	}
	for _, spec := range all {
		gid := spec.ID
		groups[gid] = m
	}

	log.Debug("Groups", "map", groups, "V", debugV3)
	return groups, nil
}

func (m *manager) loadGroupSpec(id group.ID) (group.Spec, error) {
	// load the config
	config := globalSpec{}

	// load the latest version -- assumption here is that it's been persisted already.
	err := config.load(m.Options.SpecStore)
	if err != nil {
		log.Warn("Error loading config", "err", err)
		return group.Spec{}, err
	}
	return config.getGroupSpec(id)
}

func (m *manager) updateConfig(spec group.Spec) error {
	log.Debug("Updating config", "spec", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}
	err := stored.load(m.Options.SpecStore)
	if err != nil {
		return err
	}

	log.Debug("Saving updated config", "global", stored, "spec", spec)
	defer log.Debug("Saved snapshot", "global", stored, "spec", spec)

	stored.updateGroupSpec(spec, m.Options.Group)
	return stored.store(m.Options.SpecStore)
}

func (m *manager) removeConfig(id group.ID) error {
	log.Debug("Removing config", "groupID", id)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}
	err := stored.load(m.Options.SpecStore)
	if err != nil {
		return err
	}
	log.Debug("Deleting config", "global", stored, "id", id)
	defer log.Debug("Saved snapshot", "global", stored, "id", id)

	stored.removeGroup(id)
	return stored.store(m.Options.SpecStore)
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.queue("commit",
		func() (bool, error) {
			log.Debug("Manager CommitGroup", "spec", grp, "V", debugV)

			// We first update the user's desired state first
			if !pretend {
				if updateErr := m.updateConfig(grp); updateErr != nil {
					log.Error("Error updating", "err", updateErr)
					resp = "Cannot update spec. Abort"
					err = updateErr
					return false, updateErr
				}
			}

			resp, err = m.Plugin.CommitGroup(grp, pretend)
			return retry, err
		})

	return
}

// Serialized describe group
func (m *manager) DescribeGroup(id group.ID) (desc group.Description, err error) {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.queue("describe",
		func() (bool, error) {
			log.Debug("Manager DescribeGroup", "id", id, "V", debugV)
			desc, err = m.Plugin.DescribeGroup(id)
			return retry, err
		})
	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) DestroyGroup(id group.ID) (err error) {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.queue("destroy",
		func() (bool, error) {
			log.Debug("Manager DestroyGroup", "groupID", id, "V", debugV)

			// We first update the user's desired state first
			if removeErr := m.removeConfig(id); removeErr != nil {
				log.Warn("Error updating/ remove", "err", removeErr)
				err = removeErr
				return retry, err
			}

			err = m.Plugin.DestroyGroup(id)
			return retry, err
		})
	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) FreeGroup(id group.ID) (err error) {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.queue("free",
		func() (bool, error) {
			log.Debug("Manager FreeGroup", "groupID", id, "V", debugV)

			// We first update the user's desired state first
			if removeErr := m.removeConfig(id); removeErr != nil {
				log.Warn("Error updating / remove", "err", removeErr)
				err = removeErr
				return retry, err
			}

			err = m.Plugin.FreeGroup(id)
			return retry, err
		})

	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) DestroyInstances(id group.ID, instances []instance.ID) (err error) {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.queue("destroyInstances",
		func() (bool, error) {
			log.Debug("Manager DestroyInstances", "groupID", id, "instances", instances, "V", debugV)

			err = m.Plugin.DestroyInstances(id, instances)
			return retry, err
		})

	return
}

// This implements/ overrides the Group Plugin interface to support single group-only operations
func (m *manager) SetSize(id group.ID, size int) error {

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		return errNotLeader
	}

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

	if is, errLeader := m.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

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
