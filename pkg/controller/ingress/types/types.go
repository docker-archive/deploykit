package types

import (
	"time"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/run/depends"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

func init() {
	depends.Register("ingress", types.InterfaceSpec(controller.InterfaceSpec), ResolveDependencies)
}

// ResolveDependencies returns a list of dependencies by parsing the opaque Properties blob.
func ResolveDependencies(spec types.Spec) (depends.Runnables, error) {
	if spec.Properties == nil {
		return nil, nil
	}

	properties := Properties{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, err
	}

	out := depends.Runnables{}
	for _, p := range properties {
		out = append(out, depends.RunnableFrom(p.L4Plugin))
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

// Validate validates the struct and can mutate the fields as necessary.
func (s *Spec) Validate() error {
	for _, r := range s.Routes {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
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
	DefaultSyncInterval = 5 * time.Second
)

// Options is the controller options
type Options struct {

	// HardSync when set to true will remove entries already in the L4 that
	// may have been added by the user out of band.  Default is false.
	HardSync bool

	// MatchByLabels figures out services by looking at swarm service labels.
	// This is normally OFF and instead we just look at publish and target ports.
	MatchByLabels bool

	// SyncInterval is how often to run the sync. The syntax is the string form
	// of Go time.Duration (e.g. 1min)
	SyncInterval types.Duration

	// SourceKeySelector is a string template for selecting the join key from
	// a source instance.Description.
	SourceKeySelector string
}

// TemplateFrom returns a template after it has un-escapes any escape sequences
func TemplateFrom(source []byte) (*template.Template, error) {
	buff := template.Unescape(source)
	return template.NewTemplate(
		"str://"+string(buff),
		template.Options{MultiPass: false, MissingKey: template.MissingKeyError},
	)
}
