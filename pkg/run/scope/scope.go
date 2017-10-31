package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// Nil is no scope
var Nil = Scope{}

// Scope provides an environment in which the necessary plugins are available
// for doing a unit of work.  The scope can be local or remote, namespaced,
// depending on implementation.  The first implementation is to simply run
// a set of steps locally on a set of required plugins.  Because the scope
// provides the plugin lookup, it can control what plugins are available.
// This is good for programmatically control access of what a piece of code
// can interact with the system.
// Scope is named scope instead of 'context' because it's much heavier weight
// and involves lots of calls across process boundaries, yet it provides
// lookup and scoping of services based on some business logical and locality
// of code.
type Scope struct {

	// Plugins returns the plugin lookup
	Plugins func() discovery.Plugins

	// InstanceResolver is for looking up an instance plugin
	Instance InstanceResolver

	// Metadata is for resolving metadata / path related queries
	Metadata MetadataResolver
}

// Work is a unit of work that is executed in the scope of the plugins
// running. When work completes, the plugins are shutdown.
type Work func(Scope) error

// MetadataCall is a struct that has all the information needed to evaluate a template metadata function
type MetadataCall struct {
	Plugin metadata.Plugin
	Name   plugin.Name
	Key    types.Path
}

// InstanceResolver resolves a string name for the plugin to instance plugin
type InstanceResolver func(n string) (instance.Plugin, error)

// MetadataResolver is a function that can resolve a path to a callable to access metadata
type MetadataResolver func(p string) (*MetadataCall, error)

// DefaultScope returns the default scope
func DefaultScope(plugins func() discovery.Plugins) Scope {
	return Scope{
		Plugins:  plugins,
		Metadata: DefaultMetadataResolver(plugins),
		Instance: DefaultInstanceResolver(plugins),
	}
}
