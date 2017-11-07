package scope

import (
	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/controller"
)

// Controller implements lookup for Controller
func (f fullScope) Controller(name string) (controller.Controller, error) {
	return DefaultControllerResolver(f)(name)
}

// DefaultControllerResolver returns a resolver
func DefaultControllerResolver(plugins func() discovery.Plugins) func(string) (controller.Controller, error) {
	return func(name string) (controller.Controller, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(pn, endpoint.Address)
	}
}
