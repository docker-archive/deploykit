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
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	loadbalancer_rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

var log = logutil.New("module", "controller/ingress")

// managed is the entity that reconciles desired routes with loadbalancers
type managed struct {
	// Leader controls whether this ingress is active or not
	manager.Leadership

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() (map[ingress.Vhost]loadbalancer.L4, error)

	// routes is a function returning the desired state of routes by vhosts
	routes func() (map[ingress.Vhost][]loadbalancer.Route, error)

	// healthChecks returns the healthchecks by vhost
	healthChecks func() (map[ingress.Vhost][]ingress.HealthCheck, error)

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

	lock gsync.RWMutex
}

func (c *managed) state() ingress.Properties {
	// TODO - compute this from results of polling
	return ingress.Properties{}
}

func (c *managed) groupPlugin(g ingress.Group) (group.Plugin, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("no lookup")
	}

	endpoint, err := c.plugins().Find(g.Plugin())
	if err != nil {
		return nil, err
	}
	return group_rpc.NewClient(endpoint.Address)
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

// Run starts the controller given the spec it needs to maintain
func (c *managed) Run(spec types.Spec) error {
	err := c.init(spec)
	if err != nil {
		return err
	}
	c.start()
	return nil
}

func (c *managed) started() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.process != nil && c.poller != nil
}

func (c *managed) start() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.process != nil && c.poller != nil {
		go c.poller.Run(context.Background())
	}
}

func (c *managed) stop() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.process != nil && c.poller != nil {
		c.poller.Stop()
	}
}
