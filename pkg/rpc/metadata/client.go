package metadata

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
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

// Adapt converts a rpc client to a Metadata plugin object
func Adapt(name plugin.Name, rpcClient rpc_client.Client) metadata.Plugin {
	return &client{name: name, client: rpcClient}
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

// List returns a list of nodes under path.
func (c client) List(path types.Path) ([]string, error) {
	return list(c.name, c.client, "Metadata.List", path)
}

// Get retrieves the metadata at path.
func (c client) Get(path types.Path) (*types.Any, error) {
	return get(c.name, c.client, "Metadata.Get", path)
}
