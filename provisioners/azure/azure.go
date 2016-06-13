package azure

import (
	"errors"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/spi"
)

// ProvisionerWith returns a provision given the runtime context and credential
func ProvisionerWith(controls spi.ProvisionControls, cred spi.Credential) (spi.Provisioner, error) {
	return new(provisioner), nil
}

type provisioner struct {
	sshKeys api.SSHKeys
}

func (p *provisioner) GetProvisionTasks() []spi.Task {
	return []spi.Task{
		machines.SSHKeyGen{Keys: p.sshKeys},
		machines.CreateInstance{Provisioner: p},
	}
}

func (p *provisioner) GetTeardownTasks() []spi.Task {
	return []spi.Task{
		machines.SSHKeyRemove{Keys: p.sshKeys},
		machines.DestroyInstance{Provisioner: p},
	}
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
