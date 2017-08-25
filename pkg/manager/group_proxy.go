package manager

import (
	"sync"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// Groups returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Groups() (map[group.ID]group.Plugin, error) {
	groups := map[group.ID]group.Plugin{
		group.ID(""): m,
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

type lateBindGroup struct {
	lock   sync.Mutex
	client group.Plugin
	finder func() (group.Plugin, error)
}

func (c *lateBindGroup) do(f func(p group.Plugin) error) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.client == nil {
		c.client, err = c.finder()
		if err != nil {
			return
		}
	}
	return f(c.client)
}

func (c *lateBindGroup) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {
	err = c.do(func(p group.Plugin) error {
		resp, err = p.CommitGroup(grp, pretend)
		return err
	})
	return
}

func (c *lateBindGroup) FreeGroup(id group.ID) (err error) {
	err = c.do(func(p group.Plugin) error {
		err = p.FreeGroup(id)
		return err
	})
	return
}

func (c *lateBindGroup) DescribeGroup(id group.ID) (desc group.Description, err error) {
	err = c.do(func(p group.Plugin) error {
		desc, err = p.DescribeGroup(id)
		return err
	})
	return
}

func (c *lateBindGroup) DestroyGroup(id group.ID) (err error) {
	err = c.do(func(p group.Plugin) error {
		err = p.DestroyGroup(id)
		return err
	})
	return
}

func (c *lateBindGroup) InspectGroups() (specs []group.Spec, err error) {
	err = c.do(func(p group.Plugin) error {
		specs, err = p.InspectGroups()
		return err
	})
	return
}

func (c *lateBindGroup) DestroyInstances(id group.ID, instances []instance.ID) (err error) {
	err = c.do(func(p group.Plugin) error {
		err = p.DestroyInstances(id, instances)
		return err
	})
	return
}

func (c *lateBindGroup) Size(id group.ID) (size int, err error) {
	err = c.do(func(p group.Plugin) error {
		size, err = p.Size(id)
		return err
	})
	return
}

func (c *lateBindGroup) SetSize(id group.ID, size int) (err error) {
	err = c.do(func(p group.Plugin) error {
		err = p.SetSize(id, size)
		return err
	})
	return
}
