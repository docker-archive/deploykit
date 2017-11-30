package client

import (
	"fmt"

	"github.com/docker/infrakit/pkg/spi"
	"sync"
)

// IsErrInterfaceNotSupported returns true if the error is because the interface is not supported.
func IsErrInterfaceNotSupported(err error) bool {
	_, is := err.(errNotSupported)
	return is
}

type errNotSupported spi.InterfaceSpec

func (e errNotSupported) Error() string {
	return fmt.Sprintf("Plugin does not support interface %v", spi.InterfaceSpec(e).Encode())
}

type handshakingClient struct {
	client *client
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

type errVersionMismatch string

// Error implements error interface
func (e errVersionMismatch) Error() string {
	return string(e)
}

// IsErrVersionMismatch return true if the error is from mismatched api versions.
func IsErrVersionMismatch(e error) bool {
	_, is := e.(errVersionMismatch)
	return is
}

func (c *handshakingClient) handshake() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.handshakeResult == nil {

		objects, err := c.client.Hello()
		if err != nil {
			return err
		}

		err = errNotSupported(c.iface)
		for iface := range objects {
			if iface.Name == c.iface.Name {
				if iface.Version == c.iface.Version {
					err = nil
					break
				} else {
					err = errVersionMismatch(fmt.Sprintf(
						"Plugin supports %s interface version %s, client requires %s",
						iface.Name,
						iface.Version,
						c.iface.Version))
				}
			}
		}

		c.handshakeResult = &handshakeResult{err: err}
	}

	return c.handshakeResult.err
}

func (c *handshakingClient) Addr() string {
	return c.client.Addr()
}

func (c *handshakingClient) Call(method string, arg interface{}, result interface{}) error {
	if err := c.handshake(); err != nil {
		return err
	}

	return c.client.Call(method, arg, result)
}
