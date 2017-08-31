package types

import (
	"time"

	"github.com/docker/infrakit/pkg/controller"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
)

func init() {
	depends.Register("ingress", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
func ResolveDependencies(spec types.Spec) ([]plugin.Name, error) {
	if spec.Properties == nil {
		return nil, nil
	}

	properties := Properties{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, err
	}

	out := []plugin.Name{}
	for _, p := range properties {
		out = append(out, p.L4Plugin)
	}
	return out, nil
}

// Properties is the properties for the ingress controller.  This struct is used to parse
// the `Properties` field of a pkg/types/Spec.
type Properties []Spec

// Spec provides a mapping of a vhost to
type Spec struct {

	// Vhost is the Vhost for the load balancer
	Vhost Vhost

	// L4Plugin is the name of the L4Plugin to lookup
	L4Plugin plugin.Name

	// RouteSources allows the specification of routes based on some specialized handlers.
	// The routes are keyed by the 'handler' name and the configuration blob are specific to the keyed
	// handler.  For example, a 'swarm' handler will dynamically generate the required routes based
	// on Docker swarm services.  These routes are added to the static routes.
	RouteSources map[string]*types.Any

	// Routes are those that are always synchronized routes that are specified in the configuration.
	Routes []loadbalancer.Route

	// Backends specify where to get the nodes of the backend pool.
	Backends BackendSpec

	// HealthChecks specify how to do health check against the backend services
	HealthChecks []loadbalancer.HealthCheck
}

// Group is a qualified plugin name. The 'type' field of the name is the group ID.
type Group plugin.Name

// ID returns the group id.
func (gs Group) ID() group.ID {
	_, t := plugin.Name(gs).GetLookupAndType()
	return group.ID(t)
}

// Plugin returns the plugin to contact
func (gs Group) Plugin() plugin.Name {
	return plugin.Name(gs)
}

// BackendSpec specifies the instances that are the backends.  They can come from groups of
// a given group controller or speccific instance ids.
type BackendSpec struct {

	// Groups are the ids of the groups managed by the group controller.
	// The plugin name is used ==> plugin name and type. type is the group id.
	Groups []Group

	// Instances are static instance ids
	Instances []instance.ID
}

// Vhost is the virtual host / domain
type Vhost string

const (
	// DefaultSyncInterval is the interval between syncing backends
	DefaultSyncInterval = 2 * time.Second
)

// Options is the controller options
type Options struct {
	HardSync          bool
	RemoveListeners   bool
	PublishAllExposed bool
	SyncInterval      time.Duration
}
