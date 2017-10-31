package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
)

// Group implements lookup for Group plugin
func (f fullScope) Group(name string) (group.Plugin, error) {
	pn := plugin.Name(name)
	endpoint, err := f().Find(pn)
	if err != nil {
		return nil, err
	}
	return rpc.NewClient(endpoint.Address)
}

// DefaultGroupResolver returns a resolver
func DefaultGroupResolver(plugins func() discovery.Plugins) func(string) (group.Plugin, error) {
	return func(name string) (group.Plugin, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(endpoint.Address)
	}
}
