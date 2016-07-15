package tasks

import (
	"github.com/docker/libmachete/provisioners/spi"
)

const (
	// CreateInstanceName is the name for the CreateInstance task.
	CreateInstanceName = "instance-create"

	// DestroyInstanceName is the name for the DestroyInstance task.
	DestroyInstanceName = "instance-destroy"
)

// CreateInstance creates an instance using a provisioner.
type CreateInstance struct {
	Provisioner spi.MachineProvisioner
}

// Name returns the task name.
func (c CreateInstance) Name() string {
	return CreateInstanceName
}

// Run creates the instance with the provisioner.
func (c CreateInstance) Run(resource spi.Resource, req spi.MachineRequest, events chan<- interface{}) error {
	createInstanceEvents, err := c.Provisioner.CreateInstance(req)
	if err != nil {
		return err
	}

	for event := range createInstanceEvents {
		events <- event
	}

	return nil
}

// DestroyInstance creates an instance using a provisioner.
type DestroyInstance struct {
	Provisioner spi.MachineProvisioner
}

// Name returns the task name.
func (d DestroyInstance) Name() string {
	return DestroyInstanceName
}

// Run destroys the instance with the provisioner.
func (d DestroyInstance) Run(resource spi.Resource, _ spi.MachineRequest, events chan<- interface{}) error {
	destroyInstanceEvents, err := d.Provisioner.DestroyInstance(resource.ID())
	if err != nil {
		return err
	}

	for event := range destroyInstanceEvents {
		events <- event
	}

	return nil
}
