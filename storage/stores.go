package storage

import (
	"github.com/docker/libmachete/ssh"
)

// Contexts handles the storage of context objects
type Contexts interface {
	Save(id ContextID, contextData interface{}) error

	List() ([]ContextID, error)

	GetContext(id ContextID, contextData interface{}) error

	Delete(id ContextID) error
}

// Machines handles storage of machine inventory.  In addition to standard fields for all machines,
// it allows provisioners to include custom data.
type Machines interface {
	Save(record MachineRecord, provisionerData interface{}) error

	List() ([]MachineID, error)

	GetRecord(id MachineID) (*MachineRecord, error)

	GetDetails(id MachineID, provisionerData interface{}) error

	Delete(id MachineID) error
}

// Credentials handles storage of identities and secrets for authenticating with third-party
// systems.
type Credentials interface {
	Save(id CredentialsID, credentialsData interface{}) error

	List() ([]CredentialsID, error)

	GetCredentials(id CredentialsID, credentialsData interface{}) error

	Delete(id CredentialsID) error
}

// Templates handles storage of template
type Templates interface {
	Save(id TemplateID, templateData interface{}) error

	List() ([]TemplateID, error)

	GetTemplate(id TemplateID, templateData interface{}) error

	Delete(id TemplateID) error
}

// Keys manage the SSH keys for a machine
type Keys interface {
	Save(id KeyID, keyPair *ssh.KeyPair) error

	List() ([]KeyID, error)

	// GetEncodedPublicKey returns the public key bytes in the OpenSSH authorized_keys format
	GetEncodedPublicKey(id KeyID) ([]byte, error)

	Delete(id KeyID) error
}
