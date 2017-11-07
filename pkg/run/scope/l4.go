package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// L4 implements the lookup for L4 loadbalancer
func (f fullScope) L4(name string) (loadbalancer.L4, error) {
	return DefaultL4Resolver(f)(name)
}

// DefaultL4Resolver returns a resolver
func DefaultL4Resolver(plugins func() discovery.Plugins) func(string) (loadbalancer.L4, error) {
	return func(name string) (loadbalancer.L4, error) {
		pn := plugin.Name(name)
		endpoint, err := plugins().Find(pn)
		if err != nil {
			return nil, err
		}
		return rpc.NewClient(pn, endpoint.Address)
	}
}
