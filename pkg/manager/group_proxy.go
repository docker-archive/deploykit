package manager

import (
	"sync"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// newProxy returns a plugin interface.  The proxy is late-binding in that
// it does not resolve plugin until a method is called.
func newProxy(finder func() (group.Plugin, error)) group.Plugin {
	return &proxy{finder: finder}
}

type proxy struct {
	lock   sync.Mutex
	client group.Plugin
	finder func() (group.Plugin, error)
}

func (c *proxy) run(f func(group.Plugin) error) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.client == nil {
		if p, err := c.finder(); err == nil {
			c.client = p
		} else {
			return err
		}
	}

	return f(c.client)
}

func (c *proxy) CommitGroup(grp group.Spec, pretend bool) (resp string, err error) {
	err = c.run(func(g group.Plugin) error {
		resp, err = g.CommitGroup(grp, pretend)
		return err
	})
	return
}

func (c *proxy) FreeGroup(id group.ID) (err error) {
	err = c.run(func(g group.Plugin) error {
		err = g.FreeGroup(id)
		return err
	})
	return
}

func (c *proxy) DescribeGroup(id group.ID) (desc group.Description, err error) {
	err = c.run(func(g group.Plugin) error {
		desc, err = g.DescribeGroup(id)
		return err
	})
	return
}

func (c *proxy) DestroyGroup(id group.ID) (err error) {
	err = c.run(func(g group.Plugin) error {
		err = g.DestroyGroup(id)
		return err
	})
	return
}

func (c *proxy) InspectGroups() (specs []group.Spec, err error) {
	err = c.run(func(g group.Plugin) error {
		specs, err = g.InspectGroups()
		return err
	})
	return
}

func (c *proxy) DestroyInstances(id group.ID, instances []instance.ID) (err error) {
	err = c.run(func(g group.Plugin) error {
		return g.DestroyInstances(id, instances)
	})
	return
}

func (c *proxy) Size(id group.ID) (size int, err error) {
	err = c.run(func(g group.Plugin) error {
		size, err = g.Size(id)
		return err
	})
	return
}

func (c *proxy) SetSize(id group.ID, size int) (err error) {
	err = c.run(func(g group.Plugin) error {
		err = g.SetSize(id, size)
		return err
	})
	return
}
