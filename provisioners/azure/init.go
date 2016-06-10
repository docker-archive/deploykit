package azure

import (
	"github.com/docker/libmachete/machines"
)

func init() {
	machines.DefaultProvisioners.Register(machines.ProvisionerBuilder{
		Name:                  ProvisionerName,
		DefaultCredential:     NewCredential,
		DefaultMachineRequest: NewMachineRequest,
		Build: ProvisionerWith,
	})
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "azure"

	// ProvisionerVersion is the version
	ProvisionerVersion = "0.0"
)
