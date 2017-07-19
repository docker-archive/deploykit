package ingress

import (
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var log = logutil.New("module", "controller/ingress")

// Controller is the entity that reconciles desired routes with loadbalancers
type Controller struct {

	// leader is a manager interface that can return whether this is running as leader
	leader manager.Leadership

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() map[Vhost]loadbalancer.L4

	// routes is a function returning the desired state of routes by vhosts
	routes func() (map[Vhost]loadbalancer.Route, error)

	// groups is a function that looks up an association of vhost to lists of group ids
	groups func() (map[Vhost][]group.ID, error)

	// configureL4 is the handler for actually configuring a L4 loadbalancer
	configureL4 func(loadbalancer.L4, []loadbalancer.Route, []HealthCheck, Options) error

	// configureBackends associates the list of instances to a loadbalancer
	configureBackends func(loadbalancer.L4, []instance.ID, Options) error
}
