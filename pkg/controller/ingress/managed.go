package ingress

import (
	"fmt"
	gsync "sync"
	"time"

	"github.com/docker/infrakit/pkg/controller"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	loadbalancer_rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "controller/ingress")

func newManaged(plugins func() discovery.Plugins,
	leader manager.Leadership) *managed {
	return &managed{
		Leadership: leader,
		plugins:    plugins,
	}
}

// managed is the entity that reconciles desired routes with loadbalancers
type managed struct {
	// Leader controls whether this ingress is active or not
	manager.Leadership

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

	plugins func() discovery.Plugins

	// Finite state machine tracking
	process      *core.Process
	stateMachine fsm.Instance

	// polling
	ticker <-chan time.Time
	poller *controller.Poller

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

	if c.plugins == nil {
		return nil, fmt.Errorf("no lookup")
	}

	if c.groupClients == nil {
		c.groupClients = map[plugin.Name]group.Plugin{}
	}

	found, has := c.groupClients[g.Plugin()]
	if !has {
		endpoint, err := c.plugins().Find(g.Plugin())
		if err != nil {
			return nil, err
		}
		cl, err := group_rpc.NewClient(endpoint.Address)
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

	if c.plugins == nil {
		return nil, fmt.Errorf("no L4 plugin %v", spec.L4Plugin)
	}

	endpoint, err := c.plugins().Find(spec.L4Plugin)
	if err != nil {
		return nil, err
	}
	return loadbalancer_rpc.NewClient(spec.L4Plugin, endpoint.Address)
}

func (c *managed) started() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.process != nil && c.poller != nil
}
