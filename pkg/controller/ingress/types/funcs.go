package types

import (
	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "ingress/types")

func l4FromSpec(plugins func() discovery.Plugins, spec Spec) (loadbalancer.L4, error) {
	ep, err := plugins().Find(spec.L4Plugin)
	if err != nil {
		return nil, err
	}
	return rpc.NewClient(spec.L4Plugin, ep.Address)
}

// L4 returns a function that can return a map of vhost and L4 objects, with the help of plugin lookup.
func (p Properties) L4(plugins func() discovery.Plugins) func() (map[Vhost]loadbalancer.L4, error) {

	return func() (result map[Vhost]loadbalancer.L4, err error) {

		for _, spec := range p {

			vhost := spec.Vhost
			l4, err := l4FromSpec(plugins, spec)
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
	for _, spec := range p {
		result[spec.Vhost] = spec.HealthChecks
	}
	return
}

// Groups returns a list of group ids by Vhost
func (p Properties) Groups() (result map[Vhost][]group.ID, err error) {
	for _, spec := range p {
		result[spec.Vhost] = spec.Backends.Groups
	}
	return
}

// InstanceIDs returns a map of static instance ids by vhost
func (p Properties) InstanceIDs() (result map[Vhost][]instance.ID, err error) {
	for _, spec := range p {
		result[spec.Vhost] = spec.Backends.Instances
	}
	return
}

// Routes returns a map of routes by vhost.  This will try to parse the Routes field of each Spec
// as loadbalancer.Route.  If parsing fails, the provided function callback is used to provide
// alternative parsing of the types.Any to give the data.
func (p Properties) Routes(alternate func(*types.Any) ([]loadbalancer.Route, error)) (result map[Vhost][]loadbalancer.Route, err error) {
	for _, spec := range p {
		if spec.Routes != nil {
			routes := []loadbalancer.Route{}
			err := spec.Routes.Decode(&routes)
			if err == nil {
				result[spec.Vhost] = routes
				continue
			}

			alt, err := alternate(spec.Routes)
			if err == nil {
				result[spec.Vhost] = alt
				continue
			}

			log.Warn("cannot determine routes", "spec", spec)
		}
	}
	return
}
