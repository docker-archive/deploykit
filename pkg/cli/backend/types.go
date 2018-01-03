package backend

import (
	"sync"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ExecFunc is the function of that backend that actually does work.
type ExecFunc func(script string, cmd *cobra.Command, args []string) error

// FlagsFunc is the function for the backend to register command line flags
type FlagsFunc func(*pflag.FlagSet)

// TemplateFunc is the type of function exported / available to the scripting template
type TemplateFunc func(scope scope.Scope, trial bool, opt ...interface{}) (ExecFunc, error)

var (
	backends = map[string]TemplateFunc{}
	flags    = map[string]FlagsFunc{}
	lock     = sync.Mutex{}
)

// Register registers a named backend.  The function parameters will be matched
// in the =% %= tags of backend specification.
func Register(funcName string, backend TemplateFunc, buildFlags FlagsFunc) {
	lock.Lock()
	defer lock.Unlock()
	backends[funcName] = backend
	flags[funcName] = buildFlags
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

// VisitFlags visits all the backends flags configurers.
func VisitFlags(visitor func(string, FlagsFunc)) {
	lock.Lock()
	defer lock.Unlock()

	for funcName, f := range flags {
		visitor(funcName, f)
	}
}
