package client

import (
	"fmt"
	"github.com/docker/infrakit/pkg/rpc/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"sync"
)

type handshakingClient struct {
	client Client
	api    spi.APISpec

	// handshakeResult handles the tri-state outcome of handshake state:
	//  - handshake has not yet completed (nil)
	//  - handshake completed successfully (non-nil result, nil error)
	//  - handshake failed (non-nil result, non-nil error)
	handshakeResult *handshakeResult

	// lock guards handshakeResult
	lock *sync.Mutex
}

type handshakeResult struct {
	err error
}

func (c *handshakingClient) handshake() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.handshakeResult == nil {
		req := plugin.APIsRequest{}
		resp := plugin.APIsResponse{}

		if err := c.client.Call("Plugin.APIs", req, &resp); err != nil {
			return err
		}

		err := fmt.Errorf("Plugin does not support API %v", c.api)
		if resp.APIs != nil {
			for _, api := range resp.APIs {
				if api.Name == c.api.Name {
					if api.Version == c.api.Version {
						err = nil
						break
					} else {
						err = fmt.Errorf(
							"Plugin supports %s API version %s, client requires %s",
							api.Name,
							api.Version,
							c.api.Version)
					}
				}
			}
		}

		c.handshakeResult = &handshakeResult{err: err}
	}

	return c.handshakeResult.err
}

func (c *handshakingClient) Call(method string, arg interface{}, result interface{}) error {
	if err := c.handshake(); err != nil {
		return err
	}

	return c.client.Call(method, arg, result)
}
