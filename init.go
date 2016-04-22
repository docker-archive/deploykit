package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"sync"
)

// Registry associates Provisioners canonical names.
type Registry struct {
	mutex        sync.Mutex
	provisioners map[string]func(map[string]string) api.Provisioner
}

// newEmptyRegistry creates a registry with no associations.
func newEmptyRegistry() *Registry {
	return &Registry{provisioners: make(map[string]func(map[string]string) api.Provisioner)}
}

var global = newEmptyRegistry()

// GetGlobalRegistry returns the static shared registry.
func GetGlobalRegistry() *Registry {
	return global
}

// Register makes a Provisioner factory available for use by a canonical short name.
// The factory function will be passed provider-specific details such as credentials.
func (r *Registry) Register(
	name string,
	factory func(map[string]string) api.Provisioner) error {

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.provisioners[name]; exists {
		return fmt.Errorf("A provisioner is already regisered with name '%s'", name)
	}

	r.provisioners[name] = factory
	return nil
}

// Get fetches a provisioner factory by name.
func (r *Registry) Get(name string, params map[string]string) api.Provisioner {
	if p, exists := r.provisioners[name]; exists {
		return p(params)
	}
	return nil
}
