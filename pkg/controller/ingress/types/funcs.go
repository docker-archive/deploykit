package types

import (
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log               = logutil.New("module", "ingress/types")
	routeHandlers     = map[string]func(*types.Any) ([]loadbalancer.Route, error){}
	routeHandlersLock = sync.Mutex{}
)

// RegisterRouteHandler registers a package specific handler for determining the L4 routes (e.g. static or swarm)
func RegisterRouteHandler(key string, f func(*types.Any) ([]loadbalancer.Route, error)) {
	routeHandlersLock.Lock()
	defer routeHandlersLock.Unlock()

	if f != nil {
		routeHandlers[key] = f
	}
}

// L4Func returns a function that can return a map of vhost and L4 objects, with the help of plugin lookup.
func (p Properties) L4Func(plugins func() discovery.Plugins,
	findL4 func(spec Spec, ep *plugin.Endpoint) (loadbalancer.L4, error)) func() (map[Vhost]loadbalancer.L4, error) {
	return p.l4Func(plugins, func(spec Spec, ep *plugin.Endpoint) (loadbalancer.L4, error) {
		return rpc.NewClient(spec.L4Plugin, ep.Address)
	})
}

func (p Properties) l4Func(plugins func() discovery.Plugins,
	findL4 func(spec Spec, ep *plugin.Endpoint) (loadbalancer.L4, error)) func() (map[Vhost]loadbalancer.L4, error) {

	return func() (result map[Vhost]loadbalancer.L4, err error) {
		result = map[Vhost]loadbalancer.L4{}
		for _, spec := range p {

			vhost := spec.Vhost

			ep, err := plugins().Find(spec.L4Plugin)
			if err != nil {
				return nil, err
			}
			if ep == nil {
				continue
			}

			l4, err := findL4(spec, ep)
			if err != nil {
				log.Warn("cannot locate L4 plugin", "vhost", vhost, "spec", spec)
				continue
			}

			result[vhost] = l4
		}
		return
	}
}

// HealthChecks returns a map of health checks by vhost
func (p Properties) HealthChecks() (result map[Vhost][]HealthCheck, err error) {
	result = map[Vhost][]HealthCheck{}
	for _, spec := range p {
		result[spec.Vhost] = spec.HealthChecks
	}
	return
}

// Groups returns a list of group ids by Vhost
func (p Properties) Groups() (result map[Vhost][]group.ID, err error) {
	result = map[Vhost][]group.ID{}
	for _, spec := range p {
		result[spec.Vhost] = spec.Backends.Groups
	}
	return
}

// InstanceIDs returns a map of static instance ids by vhost
func (p Properties) InstanceIDs() (result map[Vhost][]instance.ID, err error) {
	result = map[Vhost][]instance.ID{}
	for _, spec := range p {
		result[spec.Vhost] = spec.Backends.Instances
	}
	return
}

// Routes returns a map of routes by vhost.  This will try to parse the Routes field of each Spec
// as loadbalancer.Route.  If parsing fails, the provided function callback is used to provide
// alternative parsing of the types.Any to give the data.
func (p Properties) Routes() (result map[Vhost][]loadbalancer.Route, err error) {
	result = map[Vhost][]loadbalancer.Route{}
	for _, spec := range p {

		routes := spec.Routes

		for key, config := range spec.RouteSources {
			handler, has := routeHandlers[key]
			if !has {
				continue
			}
			more, err := handler(config)
			if err != nil {
				continue
			}

			routes = append(routes, more...)
		}

		result[spec.Vhost] = routes
	}
	return
}
