package manager

import (
	"sync"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// Groups returns a map of *scoped* group controllers by ID of the group.
func (m *manager) Groups() (map[group.ID]group.Plugin, error) {
	groups := map[group.ID]group.Plugin{
		group.ID(""): m.Plugin,
	}
	all, err := m.Plugin.InspectGroups()
	if err != nil {
		return groups, nil
	}
	for _, spec := range all {
		gid := spec.ID
		groups[gid] = m.Plugin
	}
	log.Debug("Groups", "map", groups, "V", debugV)
	return groups, nil
}

type proxy struct {
	lock   sync.Mutex
	client group.Plugin
	finder func() (group.Plugin, error)
}

type pluginHelper interface {
	getPlugin() (group.Plugin, error)
}

func (p *proxy) getPlugin() (group.Plugin, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.client != nil {
		return p.client, nil
	}
	return p.finder()
}

// newGroupProxy returns a plugin interface.  The proxy is late-binding in that
// it does not resolve plugin until a method is called.
func newGroupProxy(finder func() (group.Plugin, error)) group.Plugin {
	return &pGroup{&proxy{finder: finder}}
}

type pGroup struct {
	pluginHelper
}

func (c *pGroup) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.CommitGroup(grp, pretend)
}

func (c *pGroup) FreeGroup(id group.ID) (err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.FreeGroup(id)
}

func (c *pGroup) DescribeGroup(id group.ID) (desc group.Description, err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.DescribeGroup(id)
}

func (c *pGroup) DestroyGroup(id group.ID) (err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.DestroyGroup(id)
}

func (c *pGroup) InspectGroups() (specs []group.Spec, err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.InspectGroups()
}

func (c *pGroup) DestroyInstances(id group.ID, instances []instance.ID) (err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.DestroyInstances(id, instances)
}

func (c *pGroup) Size(id group.ID) (size int, err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.Size(id)
}

func (c *pGroup) SetSize(id group.ID, size int) (err error) {
	var p group.Plugin
	p, err = c.getPlugin()
	if err != nil {
		return
	}
	return p.SetSize(id, size)
}
