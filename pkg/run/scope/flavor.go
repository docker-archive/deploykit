package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	"github.com/docker/infrakit/pkg/spi/flavor"
)

// Flavor implements lookup for Flavor plugins
func (f fullScope) Flavor(name string) (flavor.Plugin, error) {
	return DefaultFlavorResolver(f)(name)
}

// DefaultFlavorResolver returns a resolver
func DefaultFlavorResolver(plugins func() discovery.Plugins) func(string) (flavor.Plugin, error) {
	return func(name string) (flavor.Plugin, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(pn, endpoint.Address)
	}
}
