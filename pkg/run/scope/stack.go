package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/manager"
	"github.com/docker/infrakit/pkg/spi/stack"
)

// Stack implements lookup for Stack plugin
func (f fullScope) Stack(name string) (stack.Interface, error) {
	return DefaultStackResolver(f)(name)
}

// DefaultStackResolver returns a resolver
func DefaultStackResolver(plugins func() discovery.Plugins) func(string) (stack.Interface, error) {
	return func(name string) (stack.Interface, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(endpoint.Address)
	}
}
