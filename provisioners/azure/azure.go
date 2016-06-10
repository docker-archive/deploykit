package azure

import (
	"errors"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
)

// ProvisionerWith returns a provision given the runtime context and credential
func ProvisionerWith(controls spi.ProvisionControls, cred spi.Credential) (spi.Provisioner, error) {
	return new(provisioner), nil
}

type provisioner struct {
	sshKeys api.SSHKeys
}

// NewMachineRequest returns a canonical machine request suitable for this provisioner.
// This includes the standard workflow steps as well as the platform attributes.
func NewMachineRequest() spi.MachineRequest {
	req := new(CreateInstanceRequest)
	req.Provisioner = ProvisionerName
	req.ProvisionerVersion = ProvisionerVersion
	req.Provision = []string{api.SSHKeyGenerateName, api.CreateInstanceName}
	req.Teardown = []string{api.SSHKeyRemoveName, api.DestroyInstanceName}
	return req
}

// Name returns the name of the provisioner
func (p *provisioner) Name() string {
	return ProvisionerName
}

func (p *provisioner) GetProvisionTasks() []spi.Task {
	return []spi.Task{
		api.SSHKeyGen(p.sshKeys),
		api.CreateInstance(p),
	}
}

func (p *provisioner) GetTeardownTasks() []spi.Task {
	return []spi.Task{
		api.SSHKeyRemove(p.sshKeys),
		api.DestroyInstance(p),
	}
}

func (p *provisioner) NewRequestInstance() spi.MachineRequest {
	return NewMachineRequest()
}

func (p *provisioner) GetInstanceID(req spi.MachineRequest) (string, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) GetIPAddress(req spi.MachineRequest) (string, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) CreateInstance(req spi.MachineRequest) (<-chan spi.CreateInstanceEvent, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) DestroyInstance(instanceID string) (<-chan spi.DestroyInstanceEvent, error) {
	panic(errors.New("not implemented"))
}
