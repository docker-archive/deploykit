package client

import (
	"fmt"
	"github.com/docker/infrakit/pkg/rpc/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"sync"
)

type handshakingClient struct {
	client Client
	iface  spi.InterfaceSpec

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
		req := plugin.ImplementsRequest{}
		resp := plugin.ImplementsResponse{}

		if err := c.client.Call("Plugin.Implements", req, &resp); err != nil {
			return err
		}

		err := fmt.Errorf("Plugin does not support interface %v", c.iface)
		if resp.APIs != nil {
			for _, iface := range resp.APIs {
				if iface.Name == c.iface.Name {
					if iface.Version == c.iface.Version {
						err = nil
						break
					} else {
						err = fmt.Errorf(
							"Plugin supports %s interface version %s, client requires %s",
							iface.Name,
							iface.Version,
							c.iface.Version)
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
