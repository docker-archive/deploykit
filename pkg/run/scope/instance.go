package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// Instance implements lookup for Instance plugin
func (f fullScope) Instance(name string) (instance.Plugin, error) {
	return DefaultInstanceResolver(f)(name)
}

// DefaultInstanceResolver returns a resolver
func DefaultInstanceResolver(plugins func() discovery.Plugins) func(string) (instance.Plugin, error) {
	return func(name string) (instance.Plugin, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(pn, endpoint.Address)
	}
}
