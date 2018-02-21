package group

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
)

// LazyConnect returns a Plugin that defers connection to actual method invocation and can optionally
// block until the connection is made.
func LazyConnect(finder func() (Plugin, error), retry time.Duration) Plugin {
	p := &lazyConnect{
		finder: finder,
		retry:  retry,
	}
	if retry > 0 {
		p.retryCancel = make(chan struct{})
	}
	return p
}

// CancelWait stops the plugin from waiting / retrying to connect
func CancelWait(p Plugin) {
	if g, is := p.(*lazyConnect); is {
		if g.retryCancel != nil {
			close(g.retryCancel)
		}
	}
}

type lazyConnect struct {
	retry       time.Duration
	retryCancel chan struct{}
	lock        sync.Mutex
	client      Plugin
	finder      func() (Plugin, error)
}

func (c *lazyConnect) do(f func(p Plugin) error) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.client == nil {
		c.client, err = c.finder()
		if err == nil && c.client != nil {
			return f(c.client)
		}

		if c.retry == 0 {
			return err
		}

		tick := time.Tick(c.retry)
		for {
			select {
			case <-tick:
			case <-c.retryCancel:
				return fmt.Errorf("cancelled")
			}

			c.client, err = c.finder()
			if err == nil && c.client != nil {
				break
			}
		}
	}
	return f(c.client)
}

func (c *lazyConnect) CommitGroup(grp Spec, pretend bool) (resp string, err error) {
	err = c.do(func(p Plugin) error {
		resp, err = p.CommitGroup(grp, pretend)
		return err
	})
	return
}

func (c *lazyConnect) FreeGroup(id ID) (err error) {
	err = c.do(func(p Plugin) error {
		err = p.FreeGroup(id)
		return err
	})
	return
}

func (c *lazyConnect) DescribeGroup(id ID) (desc Description, err error) {
	err = c.do(func(p Plugin) error {
		desc, err = p.DescribeGroup(id)
		return err
	})
	return
}

func (c *lazyConnect) DestroyGroup(id ID) (err error) {
	err = c.do(func(p Plugin) error {
		err = p.DestroyGroup(id)
		return err
	})
	return
}

func (c *lazyConnect) InspectGroups() (specs []Spec, err error) {
	err = c.do(func(p Plugin) error {
		specs, err = p.InspectGroups()
		return err
	})
	return
}

func (c *lazyConnect) DestroyInstances(id ID, instances []instance.ID) (err error) {
	err = c.do(func(p Plugin) error {
		err = p.DestroyInstances(id, instances)
		return err
	})
	return
}

func (c *lazyConnect) Size(id ID) (size int, err error) {
	err = c.do(func(p Plugin) error {
		size, err = p.Size(id)
		return err
	})
	return
}

func (c *lazyConnect) SetSize(id ID, size int) (err error) {
	err = c.do(func(p Plugin) error {
		err = p.SetSize(id, size)
		return err
	})
	return
}
