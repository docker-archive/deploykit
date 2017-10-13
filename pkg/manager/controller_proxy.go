package manager

import (
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/controller/group"
	"github.com/docker/infrakit/pkg/core"
)

// GroupControllers returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Controllers() (map[string]controller.Controller, error) {
	controllers := map[string]controller.Controller{
		".": group.AsController(core.NewAddressable("group", m.Options.Name, ""), m),
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
	log.Debug("Controllers", "map", controllers, "V", debugV2)
	return controllers, nil
}
