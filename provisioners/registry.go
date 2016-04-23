package provisioners

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
)

// Registry associates Provisioners canonical names.
type Registry struct {
	provisioners map[string]Creator
}

// A Creator retrieves a provisioner using the provided parameters.
type Creator interface {
	Create(params map[string]string) (api.Provisioner, error)
}

// NewRegistry creates a registry with the provided associations from canonical short name to a
// factory function that will create a provisioner on demand.
func NewRegistry(provisioners map[string]Creator) *Registry {
	return &Registry{provisioners: provisioners}
}

// Get fetches a provisioner by name.
func (r *Registry) Get(name string, params map[string]string) (api.Provisioner, error) {
	if p, exists := r.provisioners[name]; exists {
		return p.Create(params)
	}
	return nil, fmt.Errorf("Provisioner '%s' does not exist", name)
}
