package backend

import (
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
)

// ExecFunc is the function of that backend that actually does work.
type ExecFunc func(script string) error

// Plugins is the function that returns the plugins list
type Plugins func() discovery.Plugins

// TemplateFunc is the type of function exported / available to the scripting template
type TemplateFunc func(plugins Plugins, opt ...interface{}) (ExecFunc, error)

var (
	backends = map[string]TemplateFunc{}
	lock     = sync.Mutex{}
)

// Register registers a named backend.  The function parameters will be matched
// in the =% %= tags of backend specification.
func Register(funcName string, backend TemplateFunc) {
	lock.Lock()
	defer lock.Unlock()
	backends[funcName] = backend
}

// Visit visits all the backends.  The visitor is a function that is given a view of
// a function name bound to a generator function
func Visit(visitor func(funcName string, backend TemplateFunc)) {
	lock.Lock()
	defer lock.Unlock()

	for funcName, backend := range backends {
		visitor(funcName, backend)
	}
}
