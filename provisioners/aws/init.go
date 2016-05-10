package aws

import (
	"github.com/docker/libmachete"
)

func init() {
	libmachete.RegisterCredentialer(ProvisionerName, NewCredential)
	libmachete.RegisterTemplateBuilder(ProvisionerName, NewMachineRequest)
}

const (
	// ProvisionerName is a unique name for this provisioner.
	// It is used in all API / CLI to identify the provisioner.
	ProvisionerName = "aws"

	// ProvisionerVersion is a version string that is used by the provisioner to track
	// version of persisted machine requests
	ProvisionerVersion = "0.1"
)
