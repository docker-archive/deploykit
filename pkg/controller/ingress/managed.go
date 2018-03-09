package ingress

import (
	gsync "sync"
	"time"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/controller/internal"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var (
	log    = logutil.New("module", "controller/ingress")
	debugV = logutil.V(500)
)

func newManaged(scp scope.Scope,
	leader func() stack.Leadership) *managed {
	return &managed{
		leader: leader,
		scope:  scp,
	}
}

// managed is the entity that reconciles desired routes with loadbalancers
type managed struct {
	// Leader controls whether this ingress is active or not
	leader func() stack.Leadership

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() (map[ingress.Vhost]loadbalancer.L4, error)

	// routes is a function returning the desired state of routes by vhosts
	routes func() (map[ingress.Vhost][]loadbalancer.Route, error)

	// healthChecks returns the healthchecks by vhost
	healthChecks func() (map[ingress.Vhost][]loadbalancer.HealthCheck, error)

	// groups is a function that looks up an association of vhost to lists of group ids
	groups func() (map[ingress.Vhost][]ingress.Group, error)

	// list of instance ids by vhost
	instanceIDs func() (map[ingress.Vhost][]instance.ID, error)

	// Options are properties controlling behavior of the controller
	options ingress.Options

	spec       types.Spec
	properties ingress.Properties

	scope scope.Scope

	// Finite state machine tracking
	process      *core.Process
	stateMachine fsm.FSM

	// polling
	ticker <-chan time.Time
	poller *internal.Poller

	groupClients     map[plugin.Name]group.Plugin
	groupClientsLock gsync.RWMutex

	lock gsync.RWMutex

	// template that we use to render with a source instance.Description to get the link Key
	sourceKeySelectorTemplate *template.Template
}

func (c *managed) state() ingress.Properties {
	// TODO - compute this from results of polling
	return ingress.Properties{}
}

func (c *managed) groupPlugin(g ingress.Group) (group.Plugin, error) {
	c.groupClientsLock.Lock()
	defer c.groupClientsLock.Unlock()

	if c.groupClients == nil {
		c.groupClients = map[plugin.Name]group.Plugin{}
	}

	found, has := c.groupClients[g.Plugin()]
	if !has {
		cl, err := c.scope.Group(g.Plugin().String())
		if err != nil {
			return nil, err
		}
		c.groupClients[g.Plugin()] = cl
		found = cl
	}
	return found, nil
}

func (c *managed) l4Client(spec ingress.Spec) (loadbalancer.L4, error) {
	log.Debug("Locating L4", "name", spec.L4Plugin)
	return c.scope.L4(spec.L4Plugin.String())
}

func (c *managed) started() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.process != nil && c.poller != nil
}

// Metadata returns an optional metadata.Plugin implementation
func (c *managed) Metadata() metadata.Plugin {
	return nil
}

// Events returns an optional events.Plugin implementation
func (c *managed) Events() event.Plugin {
	return nil
}

// CurrentSpec returns the spec this collection is enforcing
func (c *managed) CurrentSpec() (s types.Spec) {
	return
}

// SetPrevSpec sets the spec that a previous version of the collection was managing.
// This gives the context to the collection so that it is able to remove resources
// that no longer are needed, for example.
func (c *managed) SetPrevSpec(s types.Spec) {
	return
}

// GetPrevSpec returns the spec the this collection continues from.
func (c *managed) GetPrevSpec() (s *types.Spec) {
	return
}
