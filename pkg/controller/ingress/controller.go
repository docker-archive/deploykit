package ingress

import (
	"time"

	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/core"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/fsm"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"golang.org/x/net/context"
)

var log = logutil.New("module", "controller/ingress")

// Controller is the entity that reconciles desired routes with loadbalancers
type Controller struct {
	// Name of the group plugin / controller to lookup
	groupPluginName plugin.Name

	// leader is a manager interface that can return whether this is running as leader
	leader manager.Leadership

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() (map[ingress.Vhost]loadbalancer.L4, error)

	// routes is a function returning the desired state of routes by vhosts
	routes func() (map[ingress.Vhost][]loadbalancer.Route, error)

	// healthChecks returns the healthchecks by vhost
	healthChecks func() (map[ingress.Vhost][]ingress.HealthCheck, error)

	// groups is a function that looks up an association of vhost to lists of group ids
	groups func() (map[ingress.Vhost][]group.ID, error)

	// list of instance ids by vhost
	instanceIDs func() map[ingress.Vhost][]instance.ID

	// Options are properties controlling behavior of the controller
	options ingress.Options

	spec ingress.Properties

	plugins func() discovery.Plugins

	// Finite state machine tracking
	process      *core.Process
	stateMachine fsm.Instance

	// polling
	ticker <-chan time.Time
	poller *Poller
}

func (c *Controller) state() ingress.Properties {
	// TODO - compute this from results of polling
	return ingress.Properties{}
}

func (c *Controller) groupPlugin() (group.Plugin, error) {
	endpoint, err := c.plugins().Find(c.groupPluginName)
	if err != nil {
		return nil, err
	}
	return group_rpc.NewClient(endpoint.Address)
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
