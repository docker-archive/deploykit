package group

import (
	"github.com/docker/infrakit/pkg/spi/group"
)

// Plugin implements group.Plugin
type Plugin struct {

	// DoCommitGroup implements CommitGroup
	DoCommitGroup func(grp group.Spec, pretend bool) (string, error)

	// DoFreeGroup implements FreeGroup
	DoFreeGroup func(id group.ID) error

	// DoDescribeGroup implements DescribeGroup
	DoDescribeGroup func(id group.ID) (group.Description, error)

	// DoDestroyGroup implements DestroyGroup
	DoDestroyGroup func(id group.ID) error

	// DoInspectGroups implements InspectGroups
	DoInspectGroups func() ([]group.Spec, error)
}

// CommitGroup commits spec for a group
func (t *Plugin) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	return t.DoCommitGroup(grp, pretend)
}

// FreeGroup releases the members of the group from management
func (t *Plugin) FreeGroup(id group.ID) error {
	return t.DoFreeGroup(id)
}

// DescribeGroup describes members of the group
func (t *Plugin) DescribeGroup(id group.ID) (group.Description, error) {
	return t.DoDescribeGroup(id)
}

// DestroyGroup destroys all members of the group
func (t *Plugin) DestroyGroup(id group.ID) error {
	return t.DoDestroyGroup(id)
}

// InspectGroups returns the specs of all groups known
func (t *Plugin) InspectGroups() ([]group.Spec, error) {
	return t.DoInspectGroups()
}
