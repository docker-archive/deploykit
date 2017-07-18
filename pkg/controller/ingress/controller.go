package ingress

import (
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var log = logutil.New("module", "controller/ingress")

// Vhost is the virtual host / domain
type Vhost string

// Options is the controller options
type Options struct {
	HardSync          bool
	RemoveListeners   bool
	HealthCheck       *HealthCheck
	PublishAllExposed bool
	Interval          time.Duration
}

// HealthCheck is the configuration for an operation to determine if a service is healthy.
type HealthCheck struct {
	Port            uint32
	Healthy         int
	Unhealthy       int
	IntervalSeconds int
	TimeoutSeconds  int
}

// Controller is the entity that reconciles desired routes with loadbalancers
type Controller struct {

	// leader is a manager interface that can return whether this is running as leader
	leader manager.Leadership

	// l4s is a function that get retrieve a map of L4 loadbalancers by name
	l4s func() map[string]loadbalancer.L4

	// routes is a function returning the desired state of routes
	routes func(string) ([]loadbalancer.Route, error)

	// backends is a function that returns a list of backends for a given named loadbalancer
	backends func(string) ([]instance.ID, error)
}
