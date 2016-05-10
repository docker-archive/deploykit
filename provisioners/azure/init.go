package azure

import (
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
)

func init() {
	libmachete.RegisterCredentialer(ProvisionerName, NewCredential)
	libmachete.RegisterTemplateBuilder(ProvisionerName, NewMachineRequest)
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "azure"
)

// TODO
func NewMachineRequest() api.MachineRequest {
	return nil
}
