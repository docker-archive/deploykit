package metadata

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
// If the backend implements Updatable then the returned Plugin can be casted to Updatable.
func NewClient(name plugin.Name, socketPath string) (metadata.Plugin, error) {
	p, err := NewClientUpdatable(name, socketPath)
	if err == nil {
		return p, nil
	}
	rpcClient, err := rpc_client.New(socketPath, metadata.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{name: name, client: rpcClient}, nil
}

// FromHandshaker returns a Plugin client.  If the backend implements
// the Updatable interface, this can be type casted to metadata.Updatable
func FromHandshaker(name plugin.Name, handshaker rpc.Handshaker) (metadata.Plugin, error) {
	cl, err := rpc_client.FromHandshaker(handshaker, metadata.UpdatableInterfaceSpec)
	if err == nil {
		return AdaptUpdatable(name, cl), nil
	}
	if err != nil {
		// try with a different interface
		cl, err = rpc_client.FromHandshaker(handshaker, metadata.InterfaceSpec)
		if err == nil {
			return Adapt(name, cl), nil
		}
	}
	return nil, fmt.Errorf("plugin named %v not a metadata.Plugin or Updatable", name)
}

// Adapt converts a rpc client to a Metadata plugin object
func Adapt(name plugin.Name, rpcClient rpc_client.Client) metadata.Plugin {
	return &client{name: name, client: rpcClient}
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

// Keys returns a list of nodes under path.
func (c client) Keys(path types.Path) ([]string, error) {
	return list(c.name, c.client, "Metadata.Keys", path)
}

// Get retrieves the metadata at path.
func (c client) Get(path types.Path) (*types.Any, error) {
	return get(c.name, c.client, "Metadata.Get", path)
}
