package swarm

import (
	"reflect"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/util/docker"
	"golang.org/x/net/context"
)

// ServiceMatcher is a swarm.Service predicate.
type ServiceMatcher func(swarm.Service) bool

func matchMaps(a, b map[string]string) bool {
	for k, v := range a {
		if k == "*" {
			return true
		}

		lv, has := b[k]
		if has {
			if (v == lv) || (v == "") {
				return true
			}
		}
	}
	return false

}

var (
	// AnyLabels matches any labels in a service
	AnyLabels = map[string]string{
		"*": "",
	}

	// AnyServices is a matcher that will match any service.
	AnyServices = func(s swarm.Service) bool {
		return true
	}
)

// MatchSpecLabels returns a matcher that matches the labels as given map.  This is an OR / ANY match
func MatchSpecLabels(kv map[string]string) ServiceMatcher {
	return func(s swarm.Service) bool {
		return matchMaps(kv, s.Spec.Labels)
	}
}

type matcher struct {
	name      string
	matchFunc ServiceMatcher
	toRoutes  func([]swarm.Service) (map[ingress.Vhost][]loadbalancer.Route, error)
}

// RoutesBuilder simplifies creation of a routes.
type RoutesBuilder struct {
	options         ingress.Options
	client          docker.APIClientCloser
	err             error
	matchers        []*matcher
	hardSync        bool
	lbSpecLabel     string
	certSpecLabel   string
	healthSpecLabel string
	lock            sync.Mutex
}

// Routes can derive a list of routes based on Docker services
type Routes struct {
	options            ingress.Options
	client             docker.APIClientCloser
	interval           time.Duration
	matchers           []*matcher
	iteration          int
	lastKnownByMatcher map[string][]swarm.Service
	lbSpecLabel        string
	certSpecLabel      string
	healthSpecLabel    string
}

// NewServiceRoutes creates a routes.
func NewServiceRoutes(client docker.APIClientCloser) *RoutesBuilder {
	return &RoutesBuilder{client: client}
}

// SetHardSyncWithLB forces the routes to do a hard sync for every service.  This is not very
// efficient when there are lots of services.
func (b *RoutesBuilder) SetHardSyncWithLB(t bool) *RoutesBuilder {
	b.hardSync = t
	return b
}

// SetOptions sets the ingress options
func (b *RoutesBuilder) SetOptions(options ingress.Options) *RoutesBuilder {
	b.options = options
	return b
}

// SetSpecLabels sets the label to look for loadbalancer spec, certifcate spec, and health path spec
func (b *RoutesBuilder) SetSpecLabels(lbSpec, certSpec, healthSpec string) *RoutesBuilder {
	b.lbSpecLabel = lbSpec
	b.certSpecLabel = certSpec
	b.healthSpecLabel = healthSpec
	return b
}

// SetCertLabel sets the label to look for the certifcate id
func (b *RoutesBuilder) SetCertLabel(certLabel *string) *RoutesBuilder {
	if certLabel != nil {
		b.certSpecLabel = *certLabel
	}
	return b
}

// SetHealthMonitorPathLabel sets the label to look for the certifcate id
func (b *RoutesBuilder) SetHealthMonitorPathLabel(healthLabel *string) *RoutesBuilder {
	if healthLabel != nil {
		b.healthSpecLabel = *healthLabel
	}
	return b
}

// AddRule adds a rule to aggregate the routes for
func (b *RoutesBuilder) AddRule(n string, m ServiceMatcher,
	toRoutes func([]swarm.Service) (map[ingress.Vhost][]loadbalancer.Route, error)) *RoutesBuilder {

	b.lock.Lock()
	defer b.lock.Unlock()

	if b.matchers == nil {
		b.matchers = []*matcher{}
	}
	b.matchers = append(b.matchers, &matcher{name: n, matchFunc: m, toRoutes: toRoutes})
	return b
}

// Build creates the routes.
func (b *RoutesBuilder) Build() (*Routes, error) {
	if b.err != nil {
		return nil, b.err
	}

	routes := &Routes{
		options:            b.options,
		client:             b.client,
		matchers:           b.matchers,
		lbSpecLabel:        b.lbSpecLabel,
		certSpecLabel:      b.certSpecLabel,
		healthSpecLabel:    b.healthSpecLabel,
		lastKnownByMatcher: map[string][]swarm.Service{},
	}

	// use defaults if nothing is set.. this will match any service, with
	// the action that generates the routes from matched services.
	if len(b.matchers) == 0 {
		routes.matchers = []*matcher{
			{
				name:      "*",
				matchFunc: AnyServices,
				toRoutes:  routes.RoutesFromServices,
			},
		}
	}
	return routes, nil
}

// RoutesFromServices analyzes the given set of services and option and produces a routes by vhost.
func (p *Routes) RoutesFromServices(services []swarm.Service) (map[ingress.Vhost][]loadbalancer.Route, error) {
	return toVhostRoutes(externalLoadBalancerListenersFromServices(
		services,
		p.options.MatchByLabels,
		p.lbSpecLabel,
		p.certSpecLabel,
		p.healthSpecLabel,
	)), nil
}

// List will return all the known routes for this Docker swarm of matching services.
func (p *Routes) List() (map[ingress.Vhost][]loadbalancer.Route, error) {

	log.Debug("Listing services from swarm")

	ctx := context.Background()
	services, err := p.client.ServiceList(ctx, types.ServiceListOptions{})

	log.Debug("Swarm serviceList", "services", services, "err", err, "matchers", p.matchers, "V", debugV)

	if err != nil {
		log.Error("Error getting swarm services", "err", err)
		return nil, err
	}

	for _, matcher := range p.matchers {

		log.Debug("running matcher", "matcher", matcher)

		found := []swarm.Service{}
		for _, s := range services {
			if matcher.matchFunc(s) {
				log.Debug("Found match", "service", s.Spec.Name, "match", matcher.name)
				found = append(found, s)
			}
		}

		lastKnown, has := p.lastKnownByMatcher[matcher.name]
		if !has {
			lastKnown = []swarm.Service{}
			p.lastKnownByMatcher[matcher.name] = lastKnown
		}

		// TODO(chungers) -- We need to support policy-based behavior with ELBs -- especially
		// for those ELBs that the user has already configured in other contexts.
		// The policy needs to be per-ELB, specified in the config file, so that we avoid
		// wiping out listeners that we don't think are represented by the services in this swarm.

		// Adding an option to do a hard sync for some cases where the lb backend eventually fails to
		// update. Basically we can't trust that the backend will eventually update successfully (for whatever
		// reason), and we'd have to treat each service as though it's found anew each time.
		// This has happened with Azure where initial update was 200 and followed by a 429.
		// This is not very efficient when there are lots of swarm services and we are basically calling the
		// backend api each run.
		if p.options.HardSync || different(lastKnown, found) || p.iteration == 0 {
			log.Debug("Found matches", "total", len(found))
			return matcher.toRoutes(found)
		}

		p.lastKnownByMatcher[matcher.name] = found
		p.iteration++
	}

	return nil, nil
}

func different(a, b []swarm.Service) bool {
	if len(a) != len(b) {
		return true
	}
	checkLabels := map[string]map[string]string{}
	checkPorts := map[string]swarm.Endpoint{}
	for _, s := range a {
		checkLabels[s.Spec.Name] = s.Spec.Labels
		checkPorts[s.Spec.Name] = s.Endpoint
	}
	for _, s := range b {
		// Check for label changes
		labels, has := checkLabels[s.Spec.Name]
		if !has {
			return true
		}
		if !reflect.DeepEqual(s.Spec.Labels, labels) {
			return true
		}
		// Check for endpoint / published ports
		ep, has := checkPorts[s.Spec.Name]
		if !has {
			return true
		}
		if !reflect.DeepEqual(s.Endpoint, ep) {
			return true
		}
	}
	return false
}
