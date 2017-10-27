package metadata

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// LazyConnect returns a metadata.Plugin that defers connection to actual method invocation and can optionally
// block until the connection is made.
func LazyConnect(finder func() (metadata.Plugin, error), retry time.Duration) metadata.Plugin {
	p := &lazyConnect{
		finder: finder,
		retry:  retry,
	}
	if retry > 0 {
		p.retryCancel = make(chan struct{})
	}
	return p
}

// UpdatableLazyConnect returns a metadata.Updatable that defers connection
// to actual method invocation and can optionally block until the connection is made.
func UpdatableLazyConnect(finder func() (metadata.Updatable, error), retry time.Duration) metadata.Updatable {
	p := &lazyConnect{
		finder: func() (metadata.Plugin, error) { return finder() },
		retry:  retry,
	}
	if retry > 0 {
		p.retryCancel = make(chan struct{})
	}
	return p
}

// CancelWait stops the plugin from waiting / retrying to connect
func CancelWait(p metadata.Plugin) {
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
	client      metadata.Plugin
	finder      func() (metadata.Plugin, error)
}

func (c *lazyConnect) do(f func(p metadata.Plugin) error) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.client == nil {

		log.Debug("looking up metadata plugin")

		c.client, err = c.finder()
		if err == nil && c.client != nil {
			return f(c.client)
		}

		if c.retry == 0 {
			log.Error("Error looking up plugin", "err", err)
			return err
		}

		log.Warn("will retry looking up metadata plugin", "retry", c.retry)
		tick := time.Tick(c.retry)
		for {
			select {
			case <-tick:
			case <-c.retryCancel:
				return fmt.Errorf("cancelled")
			}

			c.client, err = c.finder()
			if err == nil && c.client != nil {
				log.Info("Connected to metadata plugin client", "client", c.client)
				break
			}
		}
	}
	return f(c.client)
}

func (c *lazyConnect) Keys(path types.Path) (child []string, err error) {
	err = c.do(func(p metadata.Plugin) error {
		child, err = p.Keys(path)
		return err
	})
	return
}

func (c *lazyConnect) Get(path types.Path) (value *types.Any, err error) {
	err = c.do(func(p metadata.Plugin) error {
		value, err = p.Get(path)
		return err
	})
	return
}

func (c *lazyConnect) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	err = c.do(func(p metadata.Plugin) error {
		if p, is := p.(metadata.Updatable); is {
			original, proposed, cas, err = p.Changes(changes)
		} else {
			err = fmt.Errorf("readonly")
		}
		return err
	})
	return
}

func (c *lazyConnect) Commit(proposed *types.Any, cas string) (err error) {
	err = c.do(func(p metadata.Plugin) error {
		if p, is := p.(metadata.Updatable); is {
			err = p.Commit(proposed, cas)
		} else {
			err = fmt.Errorf("readonly")
		}
		return err
	})
	return
}
