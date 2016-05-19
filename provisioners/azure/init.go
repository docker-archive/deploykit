package azure

import (
	"github.com/docker/libmachete"
)

func init() {
	libmachete.RegisterProvisioner(libmachete.ProvisionerBuilder{
		Name:                  ProvisionerName,
		DefaultCredential:     NewCredential(),
		DefaultMachineRequest: NewMachineRequest(),
		BuildContext:          nil,
		Build:                 ProvisionerWith,
	})
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "azure"

	// ProvisionerVersion is the version
	ProvisionerVersion = "0.0"
)
