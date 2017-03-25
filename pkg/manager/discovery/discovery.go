package discovery

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/rpc/client"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
)

type errNotFound string

func (e errNotFound) Error() string {
	return string(e)
}

// IsNotFound returns true if the error given indicates no manager is running.
func IsNotFound(e error) bool {
	_, is := e.(errNotFound)
	return is
}

// Locate looks for the plugin that implements the Manager interface and returns a client.
func Locate(plugins func() discovery.Plugins) (manager.Manager, error) {
	// Scan for a manager
	pm, err := plugins().List()
	if err != nil {
		return nil, err
	}

	for _, endpoint := range pm {
		rpcClient, err := client.New(endpoint.Address, manager.InterfaceSpec)
		if err == nil {
			return manager_rpc.Adapt(rpcClient), nil
		}
	}
	return nil, errNotFound("manager not found")
}
