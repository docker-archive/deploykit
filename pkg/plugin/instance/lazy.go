package instance

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// LazyConnect returns an instance.Plugin that defers connection to actual method invocation and can optionally
// block until the connection is made.
func LazyConnect(finder func() (instance.Plugin, error), retry time.Duration) instance.Plugin {
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
func CancelWait(p instance.Plugin) {
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
	client      instance.Plugin
	finder      func() (instance.Plugin, error)
}

func (c *lazyConnect) do(f func(p instance.Plugin) error) (err error) {
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

// Validate performs local validation on a provision request.
func (c *lazyConnect) Validate(req *types.Any) (err error) {
	err = c.do(func(p instance.Plugin) error {
		err = p.Validate(req)
		return err
	})
	return
}

// Provision creates a new instance based on the spec.
func (c *lazyConnect) Provision(spec instance.Spec) (id *instance.ID, err error) {
	err = c.do(func(p instance.Plugin) error {
		id, err = p.Provision(spec)
		return err
	})
	return
}

// Label labels the instance
func (c *lazyConnect) Label(id instance.ID, labels map[string]string) (err error) {
	err = c.do(func(p instance.Plugin) error {
		err = p.Label(id, labels)
		return err
	})
	return
}

// Destroy terminates an existing instance.
func (c *lazyConnect) Destroy(id instance.ID, context instance.Context) (err error) {
	err = c.do(func(p instance.Plugin) error {
		err = p.Destroy(id, context)
		return err
	})
	return
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
// The properties flag indicates the client is interested in receiving details about each instance.
func (c *lazyConnect) DescribeInstances(labels map[string]string,
	properties bool) (descs []instance.Description, err error) {
	err = c.do(func(p instance.Plugin) error {
		descs, err = p.DescribeInstances(labels, properties)
		return err
	})
	return
}
