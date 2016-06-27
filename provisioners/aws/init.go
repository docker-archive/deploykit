package aws

import (
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
)

func init() {
	machines.DefaultProvisioners.Register(machines.ProvisionerBuilder{
		Name:                  ProvisionerName,
		DefaultCredential:     NewCredential,
		DefaultMachineRequest: newMachineRequest,
		Build: ProvisionerWith,
	})
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
	ProvisionerName = "aws"

	// ProvisionerVersion is a version string that is used by the provisioner to track
	// version of persisted machine requests
	ProvisionerVersion = "0.1"
)
