package manager

import (
	"github.com/docker/infrakit/pkg/controller/group"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/types"
)

// GroupControllers returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Controllers() (map[string]controller.Controller, error) {

	gcontroller := group.AsController(plugin.NewAddressable("group", m.Options.Name.LookupOnly(), ""), m)
	controllers := map[string]controller.Controller{
		"groups": gcontroller,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return controllers, nil
	}
	for _, spec := range all {
		gid := spec.ID
		controllers[string(gid)] = group.AsController(
			plugin.NewAddressable("group", m.Options.Name, string(gid)), m)
	}

	for _, c := range m.Options.Controllers {

		pn := c
		control, err := m.scope.Controller(pn.String())
		if err != nil {
			return nil, err
		}

		controllers[pn.String()] = controllerAdapter{
			name:    c,
			manager: m,
			backend: control,
		}
	}
	log.Debug("Controllers", "map", controllers, "V", debugV3)
	return controllers, nil
}

func (m *manager) updateSpec(spec types.Spec, handler plugin.Name) error {
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

	stored.updateSpec(spec, handler)
	return stored.store(m.Options.SpecStore)
}

func (m *manager) removeSpec(spec types.Spec) error {
	log.Debug("Removing config", "metadata", spec)
	m.lock.Lock()
	defer m.lock.Unlock()

	// Always read and then update with the current value.  Assumes the user's input
	// is always authoritative.
	stored := globalSpec{}
	err := stored.load(m.Options.SpecStore)
	if err != nil {
		return err
	}
	log.Debug("Deleting config", "global", stored, "spec", spec)
	defer log.Debug("Saved snapshot", "global", stored, "spec", spec)

	stored.removeSpec(spec.Kind, spec.Metadata)
	return stored.store(m.Options.SpecStore)
}

type controllerAdapter struct {
	name    plugin.Name
	manager *manager
	backend controller.Controller
}

// Plan implements Controller.Plan
func (m controllerAdapter) Plan(op controller.Operation,
	spec types.Spec) (object types.Object, plan controller.Plan, err error) {

	if is, errLeader := m.manager.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.manager.queue("plan",
		func() (bool, error) {

			object, plan, err = m.backend.Plan(op, spec)
			return retry, err
		})
	return
}

// Commit implements Controller.Commit
func (m controllerAdapter) Commit(op controller.Operation, spec types.Spec) (object types.Object, err error) {

	if is, errLeader := m.manager.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	switch op {
	case controller.Enforce:
		err = m.manager.updateSpec(spec, m.name)
	case controller.Destroy:
		err = m.manager.removeSpec(spec)
	}
	if err != nil {
		return
	}

	retry := false
	<-m.manager.queue("commit",
		func() (bool, error) {
			object, err = m.backend.Commit(op, spec)
			return retry, err
		})
	return
}

// Describe implements Controller.Describe
func (m controllerAdapter) Describe(search *types.Metadata) (found []types.Object, err error) {

	if is, errLeader := m.manager.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.manager.queue("describe",
		func() (bool, error) {

			log.Debug("describe", "search", search, "V", debugV2)

			found, err = m.backend.Describe(search)
			return retry, err
		})
	return
}

// Free implements Controller.Free
func (m controllerAdapter) Free(search *types.Metadata) (found []types.Object, err error) {

	if is, errLeader := m.manager.IsLeader(); errLeader != nil || !is {
		err = errNotLeader
		return
	}

	retry := false
	<-m.manager.queue("free",
		func() (bool, error) {
			found, err = m.backend.Free(search)
			return retry, err
		})
	return
}
