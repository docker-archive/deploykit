package scope

import (
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/controller"
	"github.com/docker/infrakit/pkg/spi/controller"
)

const defaultPluginPollInterval = 2 * time.Second

// Controller implements lookup for Controller
func (f fullScope) Controller(name string) (controller.Controller, error) {
	return controller.LazyConnect(
		func() (controller.Controller, error) {
			log.Debug("looking up controller backend", "name", name)
			return DefaultControllerResolver(f)(name)
		}, defaultPluginPollInterval), nil
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
