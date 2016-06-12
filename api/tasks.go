package api

import (
	"github.com/docker/libmachete/provisioners/spi"
)

const (
	// CreateInstanceName is the name for the CreateInstance task.
	CreateInstanceName = "instance-create"

	// DestroyInstanceName is the name for the DestroyInstance task.
	DestroyInstanceName = "instance-destroy"

	// SSHKeyGenerateName is the name for the SSHKeyGen task.
	SSHKeyGenerateName = "ssh-key-generate"

	// SSHKeyRemoveName is the name for the SSHKeyRemove task.
	SSHKeyRemoveName = "ssh-key-remove"
)

// CreateInstance creates an instance using a provisioner.
type CreateInstance struct {
	Provisioner spi.Provisioner
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
	Provisioner spi.Provisioner
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

// SSHKeyGen generates and locally stores an SSH key.
type SSHKeyGen struct {
	Keys SSHKeys
}

// Name returns the task name.
func (s SSHKeyGen) Name() string {
	return SSHKeyGenerateName
}

// Run generates and saves an SSH key.
func (s SSHKeyGen) Run(resource spi.Resource, _ spi.MachineRequest, _ chan<- interface{}) error {
	key := resource.Name()
	if key == "" {
		return NewError(ErrBadInput, "Invalid resource name")
	}
	return s.Keys.NewKeyPair(SSHKeyID(key))
}

// SSHKeyRemove destroys a locally-saved SSH key.
type SSHKeyRemove struct {
	Keys SSHKeys
}

// Name returns the task name.
func (s SSHKeyRemove) Name() string {
	return SSHKeyRemoveName
}

// Run removes an SSH key.
func (s SSHKeyRemove) Run(resource spi.Resource, _ spi.MachineRequest, _ chan<- interface{}) error {
	key := resource.Name()
	if key == "" {
		return NewError(ErrBadInput, "Invalid resource name")
	}
	return s.Keys.Remove(SSHKeyID(key))
}
