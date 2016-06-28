package tasks

import (
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
)

const (
	// SSHKeyGenerateName is the name for the SSHKeyGen task.
	SSHKeyGenerateName = "ssh-key-generate"

	// SSHKeyRemoveName is the name for the SSHKeyRemove task.
	SSHKeyRemoveName = "ssh-key-remove"
)

// SSHKeyGen generates and locally stores an SSH key.
type SSHKeyGen struct {
	Keys api.SSHKeys
}

// Name returns the task name.
func (s SSHKeyGen) Name() string {
	return SSHKeyGenerateName
}

// Run generates and saves an SSH key.
func (s SSHKeyGen) Run(resource spi.Resource, _ spi.MachineRequest, _ chan<- interface{}) error {
	key := resource.Name()
	if key == "" {
		return api.NewError(api.ErrBadInput, "Invalid resource name")
	}
	return s.Keys.NewKeyPair(api.SSHKeyID(key))
}

// SSHKeyRemove destroys a locally-saved SSH key.
type SSHKeyRemove struct {
	Keys api.SSHKeys
}

// Name returns the task name.
func (s SSHKeyRemove) Name() string {
	return SSHKeyRemoveName
}

// Run removes an SSH key.
func (s SSHKeyRemove) Run(resource spi.Resource, _ spi.MachineRequest, _ chan<- interface{}) error {
	key := resource.Name()
	if key == "" {
		return api.NewError(api.ErrBadInput, "Invalid resource name")
	}
	return s.Keys.Remove(api.SSHKeyID(key))
}
