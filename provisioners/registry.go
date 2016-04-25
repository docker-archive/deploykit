package provisioners

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
)

// Registry associates Provisioners canonical names.
type Registry struct {
	provisioners map[string]ProvisionerBuilder
}

// A ProvisionerBuilder retrieves a provisioner using the provided parameters.
type ProvisionerBuilder interface {
	Build(params map[string]string) (api.Provisioner, error)
}

// NewRegistry creates a registry with the provided associations from canonical short name to a
// factory function that will create a provisioner on demand.
func NewRegistry(provisioners map[string]ProvisionerBuilder) *Registry {
	return &Registry{provisioners: provisioners}
}

// Get fetches a provisioner by name.
func (r *Registry) Get(name string, params map[string]string) (api.Provisioner, error) {
	if p, exists := r.provisioners[name]; exists {
		return p.Build(params)
	}
	return nil, fmt.Errorf("Provisioner '%s' does not exist", name)
}
