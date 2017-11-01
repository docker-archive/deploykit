package base

import (
	"sync"

	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/spf13/cobra"
)

type module func(scope.Scope) *cobra.Command

var (
	lock sync.Mutex

	modules = []module{}
)

// Register registers a command from the modules
func Register(f module) {

	lock.Lock()
	defer lock.Unlock()

	modules = append(modules, f)
}

// VisitModules iterate through all the modules known
func VisitModules(scope scope.Scope, f func(*cobra.Command)) {
	lock.Lock()
	defer lock.Unlock()

	for _, m := range modules {
		command := m(scope)
		f(command)
	}
}
