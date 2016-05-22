package azure

import (
	"errors"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"golang.org/x/net/context"
)

// ProvisionerWith returns a provision given the runtime context and credential
func ProvisionerWith(ctx context.Context, cred api.Credential) (api.Provisioner, error) {
	return new(provisioner), nil
}

type provisioner struct {
}

// NewMachineRequest returns a canonical machine request suitable for this provisioner.
// This includes the standard workflow steps as well as the platform attributes.
func NewMachineRequest() api.MachineRequest {
	req := new(CreateInstanceRequest)
	req.Provisioner = ProvisionerName
	req.ProvisionerVersion = ProvisionerVersion
	req.Provision = getProvisionTaskMap().Names()
	req.Teardown = getTeardownTaskMap().Names()
	return req
}

func getProvisionTaskMap() *libmachete.TaskMap {
	return libmachete.NewTaskMap(
		libmachete.TaskSSHKeyGen,
		libmachete.TaskCreateInstance,
		libmachete.TaskUserData,
		libmachete.TaskInstallDockerEngine,
	)
}

func getTeardownTaskMap() *libmachete.TaskMap {
	return libmachete.NewTaskMap(
		libmachete.TaskDestroyInstance,
	)
}

func (p *provisioner) GetProvisionTasks(tasks []api.TaskName) ([]api.Task, error) {
	return getProvisionTaskMap().Filter(tasks)
}

func (p *provisioner) GetTeardownTasks(tasks []api.TaskName) ([]api.Task, error) {
	return getTeardownTaskMap().Filter(tasks)
}

func (p *provisioner) NewRequestInstance() api.MachineRequest {
	return NewMachineRequest()
}

func (p *provisioner) GetInstanceID(req api.MachineRequest) (string, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) GetIPAddress(req api.MachineRequest) (string, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) CreateInstance(req api.MachineRequest) (<-chan api.CreateInstanceEvent, error) {
	panic(errors.New("not implemented"))
}

func (p *provisioner) DestroyInstance(instanceID string) (<-chan api.DestroyInstanceEvent, error) {
	panic(errors.New("not implemented"))
}
