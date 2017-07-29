package types

import (
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

// Properties is the properties for the ingress controller.  This struct is used to parse
// the `Properties` field of a pkg/types/Spec.
type Properties []Spec

// Spec provides a mapping of a vhost to
type Spec struct {

	// Vhost is the Vhost for the load balancer
	Vhost Vhost

	// L4Plugin is the name of the L4Plugin to lookup
	L4Plugin plugin.Name

	// Routes is a specification of the actual routes (port, protocol).  It's an Any
	// bacause different implmeentations (e.g. swarm) can have different configurations.
	Routes *types.Any

	// Backends specify where to get the nodes of the backend pool.
	Backends BackendSpec

	// HealthChecks specify how to do health check against the backend services
	HealthChecks []HealthCheck
}

// BackendSpec specifies the instances that are the backends.  They can come from groups of
// a given group controller or speccific instance ids.
type BackendSpec struct {

	// Groups are the ids of the groups managed by the group controller
	Groups []group.ID

	// Instances are static instance ids
	Instances []instance.ID
}

// Vhost is the virtual host / domain
type Vhost string

// Options is the controller options
type Options struct {
	HardSync          bool
	RemoveListeners   bool
	PublishAllExposed bool
	SyncInterval      time.Duration
}

// HealthCheck is the configuration for an operation to determine if a service is healthy.
type HealthCheck struct {
	Port            uint32
	Healthy         int
	Unhealthy       int
	IntervalSeconds int
	TimeoutSeconds  int
}
