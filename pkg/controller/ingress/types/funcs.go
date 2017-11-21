package types

import (
	"io"
	"sync"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log               = logutil.New("module", "controller/ingress/types")
	debugV            = logutil.V(300)
	routeHandlers     = map[string]func() (RouteHandler, error){}
	routeHandlersLock = sync.Mutex{}
)

// RouteHandler is the interface that different modules must support
type RouteHandler interface {
	io.Closer
	// Routes returns a map of vhost and loadbalancer routes given the input blob
	Routes(*types.Any, Options) (map[Vhost][]loadbalancer.Route, error)
}

// RegisterRouteHandler registers a package specific handler for determining the L4 routes (e.g. static or swarm)
func RegisterRouteHandler(key string, f func() (RouteHandler, error)) {

	routeHandlersLock.Lock()
	defer routeHandlersLock.Unlock()

	if f != nil {
		routeHandlers[key] = f
	}
}

// L4Func returns a function that can return a map of vhost and L4 objects, with the help of plugin lookup.
func (p Properties) L4Func(findL4 func(spec Spec) (loadbalancer.L4, error)) func() (map[Vhost]loadbalancer.L4, error) {

	return func() (result map[Vhost]loadbalancer.L4, err error) {
		result = map[Vhost]loadbalancer.L4{}
		for _, spec := range p {

			vhost := spec.Vhost

			l4, err := findL4(spec)
			if err != nil || l4 == nil {
				log.Warn("cannot locate L4 plugin", "vhost", vhost, "spec", spec, "err", err)
				continue
			}

			result[vhost] = l4
		}
		return
	}
}

// HealthChecks returns a map of health checks by vhost
func (p Properties) HealthChecks() (result map[Vhost][]loadbalancer.HealthCheck, err error) {
	result = map[Vhost][]loadbalancer.HealthCheck{}
	for _, spec := range p {
		result[spec.Vhost] = spec.HealthChecks
	}
	return
}

// Groups returns a list of group ids by Vhost
func (p Properties) Groups() (result map[Vhost][]Group, err error) {
	result = map[Vhost][]Group{}
	for _, spec := range p {
		log.Debug("found spec", "spec", spec, "vhost", spec.Vhost)
		if _, has := result[spec.Vhost]; !has {
			result[spec.Vhost] = []Group{}
		}
		result[spec.Vhost] = append(result[spec.Vhost], spec.Backends.Groups...)
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
func (p Properties) Routes(options Options) (result map[Vhost][]loadbalancer.Route, err error) {
	result = map[Vhost][]loadbalancer.Route{}
	for _, spec := range p {

		if err := spec.Validate(); err != nil {
			return nil, err
		}

		result[spec.Vhost] = spec.Routes

		for key, config := range spec.RouteSources {
			handlerFunc, has := routeHandlers[key]

			log.Debug("route handler", "key", key, "exists", has, "V", debugV)
			if !has {
				continue
			}

			log.Debug("calling route handler", "config", config, "options", options, "V", debugV)

			handler, err := handlerFunc()
			if err != nil {
				return nil, err
			}
			defer handler.Close()

			vhostRoutes, err := handler.Routes(config, options)

			log.Debug("found routes", "routesByVhost", vhostRoutes, "err", err)
			if err != nil {
				continue
			}

			for h, r := range vhostRoutes {
				result[h] = append(result[h], r...)
			}
		}
	}
	return
}
