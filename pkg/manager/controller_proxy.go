package manager

import (
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/controller/group"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/plugin"
	controller_rpc "github.com/docker/infrakit/pkg/rpc/controller"
	"github.com/docker/infrakit/pkg/types"
)

// GroupControllers returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Controllers() (map[string]controller.Controller, error) {

	gcontroller := group.AsController(core.NewAddressable("group", m.Options.Name.LookupOnly(), ""), m)
	controllers := map[string]controller.Controller{
		//		".":      gcontroller,
		"groups": gcontroller,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return controllers, nil
	}
	for _, spec := range all {
		gid := spec.ID
		controllers[string(gid)] = group.AsController(
			core.NewAddressable("group", m.Options.Name, string(gid)), m)
	}

	for _, c := range m.Options.Controllers {

		pn := c
		controllers[pn.String()] = controllerAdapter{
			name:    c,
			manager: m,
			backend: controller.LazyConnect(
				func() (controller.Controller, error) {

					log.Debug("looking up controller backend", "name", pn)
					endpoint, err := m.Options.Plugins().Find(pn)
					if err != nil {
						return nil, err
					}
					return controller_rpc.NewClient(pn, endpoint.Address)

				}, defaultPluginPollInterval),
		}
	}
	log.Debug("Controllers", "map", controllers, "V", debugV2)
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

func (m controllerAdapter) queue(name string, work func() error) <-chan struct{} {
	wait := make(chan struct{})
	m.manager.backendOps <- backendOp{
		name: name,
		operation: func() error {
			err := work()
			close(wait)
			return err
		},
	}
	return wait
}

// Plan implements Controller.Plan
func (m controllerAdapter) Plan(op controller.Operation, spec types.Spec) (o types.Object, p controller.Plan, e error) {
	<-m.queue("plan",
		func() error {

			o, p, e = m.backend.Plan(op, spec)
			return e
		})
	return
}

// Commit implements Controller.Commit
func (m controllerAdapter) Commit(op controller.Operation, spec types.Spec) (obj types.Object, err error) {
	<-m.queue("commit",
		func() error {

			switch op {
			case controller.Enforce:
				err = m.manager.updateSpec(spec, m.name)
			case controller.Destroy:
				err = m.manager.removeSpec(spec)
			}
			if err != nil {
				return err
			}

			obj, err = m.backend.Commit(op, spec)
			return err
		})
	return
}

// Describe implements Controller.Describe
func (m controllerAdapter) Describe(search *types.Metadata) (found []types.Object, err error) {
	<-m.queue("describe",
		func() error {

			log.Debug("describe", "search", search, "V", debugV2)

			found, err = m.backend.Describe(search)
			return err
		})
	return
}

// Free implements Controller.Free
func (m controllerAdapter) Free(search *types.Metadata) (found []types.Object, err error) {
	<-m.queue("free",
		func() error {

			found, err = m.backend.Free(search)
			return err
		})
	return
}
