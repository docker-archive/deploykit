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
)
