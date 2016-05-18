package azure

import (
	"github.com/docker/libmachete"
)

func init() {
	libmachete.RegisterCredentialer(ProvisionerName, NewCredential)
	libmachete.RegisterTemplateBuilder(ProvisionerName, NewMachineRequest)
	libmachete.RegisterMachineRequestBuilder(ProvisionerName, NewMachineRequest)
	libmachete.RegisterProvisionerBuilder(ProvisionerName, ProvisionerWith)
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "azure"

	// ProvisionerVersion is the version
	ProvisionerVersion = "0.0"
)
