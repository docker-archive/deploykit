package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
	"sync"
)

var (
	// DefaultProvisioners is a global collection of machine provisioners.
	DefaultProvisioners = MachineProvisioners{builders: make(map[string]ProvisionerBuilder)}
)

// ProvisionerBuilder defines structures needed for the provisioner to operate, and is capable
// of constructing a provisioner.
type ProvisionerBuilder struct {
	Name                  string
	DefaultCredential     api.Credential
	DefaultMachineRequest api.MachineRequest
	Build                 func(controls api.ProvisionControls, cred api.Credential) (api.Provisioner, error)
}

// MachineProvisioners maintains the collection of available provisioners.
type MachineProvisioners struct {
	builders     map[string]ProvisionerBuilder
	buildersLock sync.Mutex
}

// GetBuilder retrieves a registered provisioner builder.
func (m *MachineProvisioners) GetBuilder(name string) (ProvisionerBuilder, bool) {
	builder, has := m.builders[name]
	return builder, has
}

// Register makes a provisioner available for other components to fetch.
func (m *MachineProvisioners) Register(builder ProvisionerBuilder) {
	m.buildersLock.Lock()
	defer m.buildersLock.Unlock()

	m.builders[builder.Name] = builder
}
