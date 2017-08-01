package ingress

import (
	"fmt"
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
	"github.com/docker/infrakit/pkg/types"
	"golang.org/x/net/context"
)

var log = logutil.New("module", "controller/ingress")

// Controller is the entity that reconciles desired routes with loadbalancers
type Controller struct {
	manager.Leadership

	// Name of the group plugin / controller to lookup
	GroupPluginName plugin.Name

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() (map[ingress.Vhost]loadbalancer.L4, error)

	// routes is a function returning the desired state of routes by vhosts
	routes func() (map[ingress.Vhost][]loadbalancer.Route, error)

	// healthChecks returns the healthchecks by vhost
	healthChecks func() (map[ingress.Vhost][]ingress.HealthCheck, error)

	// groups is a function that looks up an association of vhost to lists of group ids
	groups func() (map[ingress.Vhost][]group.ID, error)

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
}

func (c *Controller) state() ingress.Properties {
	// TODO - compute this from results of polling
	return ingress.Properties{}
}

func (c *Controller) groupPlugin() (group.Plugin, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("no group plugin %v", c.GroupPluginName)
	}

	endpoint, err := c.plugins().Find(c.GroupPluginName)
	if err != nil {
		return nil, err
	}
	return group_rpc.NewClient(endpoint.Address)
}

func (c *Controller) l4Client(spec ingress.Spec) (loadbalancer.L4, error) {
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
func (c *Controller) Run(spec types.Spec) error {
	err := c.init(spec)
	if err != nil {
		return err
	}
	c.start()
	return nil
}

func (c *Controller) start() {
	if c.process != nil && c.poller != nil {
		go c.poller.Run(context.Background())
	}
}

func (c *Controller) stop() {
	if c.process != nil && c.poller != nil {
		c.poller.Stop()
	}
}
