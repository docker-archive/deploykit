package azure

import (
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
)

func init() {
	machines.DefaultProvisioners.Register(machines.ProvisionerBuilder{
		Name:                  ProvisionerName,
		DefaultCredential:     newCredential,
		DefaultMachineRequest: newMachineRequest,
		Build: ProvisionerWith,
	})
}

func newCredential() spi.Credential {
	return &credential{CredentialBase: spi.CredentialBase{}}
}

func newMachineRequest() spi.MachineRequest {
	return &createInstanceRequest{
		BaseMachineRequest: spi.BaseMachineRequest{
			Provisioner:        ProvisionerName,
			ProvisionerVersion: ProvisionerVersion,
			Provision:          []string{machines.SSHKeyGenerateName, machines.CreateInstanceName},
			Teardown:           []string{machines.SSHKeyRemoveName, machines.DestroyInstanceName},
		},
	}
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "azure"

	// ProvisionerVersion is the version
	ProvisionerVersion = "0.0"
)
