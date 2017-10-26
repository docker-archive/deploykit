package scope

import (
	"github.com/docker/infrakit/pkg/discovery"
)

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
}

// Work is a unit of work that is executed in the scope of the plugins
// running. When work completes, the plugins are shutdown.
type Work func(Scope) error
