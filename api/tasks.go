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
func CreateInstance(provisioner spi.Provisioner) spi.Task {
	handler := func(_ spi.Resource, req spi.MachineRequest, events chan<- interface{}) error {
		createInstanceEvents, err := provisioner.CreateInstance(req)
		if err != nil {
			return err
		}

		for event := range createInstanceEvents {
			events <- event
		}

		return nil
	}

	return spi.Task{
		Name:    CreateInstanceName,
		Message: "Creates a machine instance",
		Do:      handler,
	}
}

// DestroyInstance creates an instance using a provisioner.
func DestroyInstance(provisioner spi.Provisioner) spi.Task {
	handler := func(resource spi.Resource, req spi.MachineRequest, events chan<- interface{}) error {
		destroyInstanceEvents, err := provisioner.DestroyInstance(resource.ID())
		if err != nil {
			return err
		}

		for event := range destroyInstanceEvents {
			events <- event
		}

		return nil
	}

	return spi.Task{
		Name:    DestroyInstanceName,
		Message: "Destroys a machine instance",
		Do:      handler,
	}
}

// SSHKeyGen generates and locally stores an SSH key.
func SSHKeyGen(keys SSHKeys) spi.Task {
	handler := func(resource spi.Resource, req spi.MachineRequest, events chan<- interface{}) error {
		key := resource.Name()
		if key == "" {
			return NewError(ErrBadInput, "Bad resource name")
		}
		return keys.NewKeyPair(SSHKeyID(key))
	}

	return spi.Task{
		Name:    SSHKeyGenerateName,
		Message: "Generating ssh key for host",
		Do:      handler,
	}
}

// SSHKeyRemove destroys a locally-saved SSH key.
func SSHKeyRemove(keys SSHKeys) spi.Task {
	handler := func(resource spi.Resource, req spi.MachineRequest, events chan<- interface{}) error {
		key := resource.Name()
		if key == "" {
			return NewError(ErrBadInput, "Bad resource name")
		}
		return keys.Remove(SSHKeyID(key))
	}

	return spi.Task{
		Name:    SSHKeyRemoveName,
		Message: "Remove ssh key for host",
		Do:      handler,
	}
}
