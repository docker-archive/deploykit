package swarm

import (
	"reflect"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
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
	run       func([]swarm.Service)
}

// PollerBuilder simplifies creation of a poller.
type PollerBuilder struct {
	client   docker.APIClientCloser
	err      error
	matchers []*matcher
	interval time.Duration
	hardSync bool
	lock     sync.Mutex
}

// RunWithContext performs an operation using the provided context.
type RunWithContext interface {
	Run(ctx context.Context) error

	Stop()
}

type poller struct {
	hardSync bool
	client   docker.APIClientCloser
	interval time.Duration
	matchers []*matcher
	stop     chan interface{}
}

// NewServicePoller creates a poller.
func NewServicePoller(client docker.APIClientCloser, interval time.Duration) *PollerBuilder {
	return &PollerBuilder{client: client, interval: interval}
}

// SetHardSyncWithLB forces the poller to do a hard sync for every service.  This is not very
// efficient when there are lots of services.
func (b *PollerBuilder) SetHardSyncWithLB(t bool) *PollerBuilder {
	b.hardSync = t
	return b
}

// AddService adds a service to poll.
func (b *PollerBuilder) AddService(n string, m ServiceMatcher, run func([]swarm.Service)) *PollerBuilder {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.matchers == nil {
		b.matchers = []*matcher{}
	}
	b.matchers = append(b.matchers, &matcher{name: n, matchFunc: m, run: run})
	return b
}

// Build creates the poller.
func (b *PollerBuilder) Build() (RunWithContext, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.err != nil {
		return nil, b.err
	}
	return &poller{
		stop:     make(chan interface{}),
		client:   b.client,
		matchers: b.matchers,
		interval: b.interval,
		hardSync: b.hardSync,
	}, nil
}

// Stop stops the poller
func (p *poller) Stop() {
	if p.stop != nil {
		close(p.stop)
	}
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p *poller) Run(ctx context.Context) error {
	ticker := time.Tick(p.interval)

	lastKnownByMatcher := map[string][]swarm.Service{}
	iteration := 0

	for {
		select {

		case <-p.stop:
			log.Infoln("Stopping poller")
			return nil

		case <-ctx.Done():
			return ctx.Err()

		case <-ticker:

		}

		services, err := p.client.ServiceList(ctx, types.ServiceListOptions{})
		if err != nil {
			return err
		}

		for _, matcher := range p.matchers {
			found := []swarm.Service{}
			for _, s := range services {
				if matcher.matchFunc(s) {
					log.Debugln("Service", s.Spec.Name, "matches", matcher.name)
					found = append(found, s)
				}
			}

			lastKnown, has := lastKnownByMatcher[matcher.name]
			if !has {
				lastKnown = []swarm.Service{}
				lastKnownByMatcher[matcher.name] = lastKnown
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
			if p.hardSync || different(lastKnown, found) || iteration == 0 {
				log.Infoln(len(found), "matches found. Processing.")
				matcher.run(found)
			}

			lastKnownByMatcher[matcher.name] = found
			iteration++
		}
	}
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
