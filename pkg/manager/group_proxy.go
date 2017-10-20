package manager

import (
	"github.com/docker/infrakit/pkg/spi/group"
)

// Groups returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Groups() (map[group.ID]group.Plugin, error) {
	groups := map[group.ID]group.Plugin{
		group.ID("."): m,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return groups, nil
	}
	for _, spec := range all {
		gid := spec.ID
		groups[gid] = m
	}
	log.Debug("Groups", "map", groups, "V", debugV2)
	return groups, nil
}
