package base

import (
	"sync"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/spf13/cobra"
)

type module func(func() discovery.Plugins) *cobra.Command

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
func VisitModules(p discovery.Plugins, f func(*cobra.Command)) {
	lock.Lock()
	defer lock.Unlock()

	for _, m := range modules {
		command := m(func() discovery.Plugins { return p })
		f(command)
	}
}
