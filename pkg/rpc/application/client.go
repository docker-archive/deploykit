package application

import (
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/types"
)

// NewClient returns a plugin interface implementation connected to a remote plugin
func NewClient(name plugin.Name, socketPath string) (application.Plugin, error) {
	rpcClient, err := rpc_client.New(socketPath, application.InterfaceSpec)
	if err != nil {
		return nil, err
	}
	return &client{name: name, client: rpcClient}, nil
}

type client struct {
	name   plugin.Name
	client rpc_client.Client
}

// Validate checks whether the helper can support a configuration.
func (c client) Validate(applicationProperties *types.Any) error {
	_, applicationType := c.name.GetLookupAndType()
	req := ValidateRequest{Type: applicationType, Properties: applicationProperties}
	resp := ValidateResponse{}
	return c.client.Call("Application.Validate", req, &resp)
}

// Healthy determines the Health of this Application on an instance.
func (c client) Healthy(applicationProperties *types.Any) (application.Health, error) {
	_, applicationType := c.name.GetLookupAndType()
	req := HealthyRequest{Type: applicationType, Properties: applicationProperties}
	resp := HealthyResponse{}
	err := c.client.Call("Application.Healthy", req, &resp)
	return resp.Health, err
}

func (c client) Update(message *application.Message) error {
	_, applicationType := c.name.GetLookupAndType()
	req := UpdateRequest{Type: applicationType, Message: message}
	resp := UpdateResponse{}
	return c.client.Call("Application.Update", req, &resp)
}
