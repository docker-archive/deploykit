package machines

import (
	"github.com/docker/libmachete/provisioners/spi"
)

var (
	// DefaultProvisioners is a global collection of machine provisioners.
	DefaultProvisioners = MachineProvisioners{builders: make(map[string]ProvisionerBuilder)}
)

// ProvisionerBuilder defines structures needed for the provisioner to operate, and is capable
// of constructing a provisioner.
// TODO(wfarner): Consider converting this to an interface.
type ProvisionerBuilder struct {
	// TODO(wfarner): This function is redundant with Provisioner.Name().
	Name              string
	DefaultCredential func() spi.Credential
	// TODO(wfarner): This function is redundant with Provisioner.NewRequestInstance().
	DefaultMachineRequest func() spi.MachineRequest
	Build                 func(controls spi.ProvisionControls, cred spi.Credential) (spi.Provisioner, error)
}

// MachineProvisioners maintains the collection of available provisioners.
type MachineProvisioners struct {
	builders map[string]ProvisionerBuilder
}

// NewMachineProvisioners creates a collection of provisioners from a slice of builders.
func NewMachineProvisioners(builders []ProvisionerBuilder) MachineProvisioners {
	m := MachineProvisioners{builders: make(map[string]ProvisionerBuilder)}
	for _, builder := range builders {
		m.Register(builder)
	}
	return m
}

// GetBuilder retrieves a registered provisioner builder.
func (m *MachineProvisioners) GetBuilder(name string) (ProvisionerBuilder, bool) {
	builder, has := m.builders[name]
	return builder, has
}

// Register makes a provisioner available for other components to fetch.
func (m *MachineProvisioners) Register(builder ProvisionerBuilder) {
	m.builders[builder.Name] = builder
}
