package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/types"
)

// LazyConnect returns a Controller that defers connection to actual method invocation and can optionally
// block until the connection is made.
func LazyConnect(finder func() (Controller, error), retry time.Duration) Controller {
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
func CancelWait(p Controller) {
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
	client      Controller
	finder      func() (Controller, error)
}

func (c *lazyConnect) do(f func(p Controller) error) (err error) {
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

func (c *lazyConnect) Plan(op Operation, spec types.Spec) (object types.Object, plan Plan, err error) {
	err = c.do(func(p Controller) error {
		object, plan, err = p.Plan(op, spec)
		return err
	})
	return
}

func (c *lazyConnect) Commit(op Operation, spec types.Spec) (object types.Object, err error) {
	err = c.do(func(p Controller) error {
		object, err = p.Commit(op, spec)
		return err
	})
	return
}

func (c *lazyConnect) Describe(search *types.Metadata) (objects []types.Object, err error) {
	err = c.do(func(p Controller) error {
		objects, err = p.Describe(search)
		return err
	})
	return
}

func (c *lazyConnect) Free(search *types.Metadata) (objects []types.Object, err error) {
	err = c.do(func(p Controller) error {
		objects, err = p.Free(search)
		return err
	})
	return
}
