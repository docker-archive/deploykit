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
}

// NewMachineRequest returns a canonical machine request suitable for this provisioner.
// This includes the standard workflow steps as well as the platform attributes.
func NewMachineRequest() spi.MachineRequest {
	req := new(CreateInstanceRequest)
	req.Provisioner = ProvisionerName
	req.ProvisionerVersion = ProvisionerVersion
	req.Provision = getProvisionTaskMap().Names()
	req.Teardown = getTeardownTaskMap().Names()
	return req
}

func getProvisionTaskMap() *api.TaskMap {
	return api.NewTaskMap(
		api.TaskSSHKeyGen,
		api.TaskCreateInstance,
		api.TaskUserData,
		api.TaskInstallDockerEngine,
	)
}

func getTeardownTaskMap() *api.TaskMap {
	return api.NewTaskMap(
		api.TaskDestroyInstance,
	)
}

// Name returns the name of the provisioner
func (p *provisioner) Name() string {
	return ProvisionerName
}

func (p *provisioner) GetProvisionTasks(tasks []spi.TaskName) ([]spi.Task, error) {
	return getProvisionTaskMap().Filter(tasks)
}

func (p *provisioner) GetTeardownTasks(tasks []spi.TaskName) ([]spi.Task, error) {
	return getTeardownTaskMap().Filter(tasks)
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
