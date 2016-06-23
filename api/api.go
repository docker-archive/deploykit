package api

import (
	"github.com/docker/libmachete/provisioners/spi"
	"io"
)

// Credentials manages the objects used to authenticate and authorize for a provisioner's API
type Credentials interface {
	// ListIds
	ListIds() ([]CredentialsID, *Error)

	// Get returns a credential identified by key
	Get(id CredentialsID) (spi.Credential, *Error)

	// Deletes the credential identified by key
	Delete(id CredentialsID) *Error

	// CreateCredential adds a new credential from the input reader.
	CreateCredential(id CredentialsID, input io.Reader, codec Codec) *Error

	// UpdateCredential updates an existing credential
	UpdateCredential(id CredentialsID, input io.Reader, codec Codec) *Error
}

// Machines manages the lifecycle of a machine / node.
type Machines interface {
	// List returns summaries of all machines.
	List() ([]MachineSummary, *Error)

	// ListIds returns the identifiers for all machines.
	ListIds() ([]MachineID, *Error)

	// Get returns a machine identified by key
	Get(id MachineID) (MachineRecord, *Error)

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(
		provisionerName string,
		credentialsName string,
		controls spi.ProvisionControls,
		templateName string,
		input io.Reader,
		codec Codec) (<-chan interface{}, *Error)

	// DeleteMachine deletes a machine.  The stored record for the machine will be used to define workflow tasks
	// performed.
	// TODO(wfarner): ProvisionControls is no longer an appropriate name since it's reused for deletion.  Leaving
	// for now as a revamp is imminent.
	DeleteMachine(
		credentialsName string,
		controls spi.ProvisionControls,
		machine MachineID) (<-chan interface{}, *Error)
}

// SSHKeys provides operations for generating and managing SSH keys.
type SSHKeys interface {
	// NewKeyPair creates and saves a new key pair identified by the id
	NewKeyPair(id SSHKeyID) error

	// GetEncodedPublicKey returns the public key bytes for the key pair identified by id.
	// The format is in the OpenSSH authorized_keys format.
	GetEncodedPublicKey(id SSHKeyID) ([]byte, error)

	// Remove the keypair
	Remove(id SSHKeyID) error

	// ListIds
	ListIds() ([]SSHKeyID, error)
}

// Templates looks up and reads template data, scoped by provisioner name.
type Templates interface {
	// NewTemplate returns a blank template, which can be used to describe the template schema.
	NewBlankTemplate(provisionerName string) (spi.MachineRequest, *Error)

	// ListIds
	ListIds() ([]TemplateID, *Error)

	// Get returns a template identified by provisioner and key
	Get(id TemplateID) (spi.MachineRequest, *Error)

	// Deletes the template identified by provisioner and key
	Delete(id TemplateID) *Error

	// CreateTemplate adds a new template from the input reader.
	CreateTemplate(id TemplateID, input io.Reader, codec Codec) *Error

	// UpdateTemplate updates an existing template
	UpdateTemplate(id TemplateID, input io.Reader, codec Codec) *Error
}
