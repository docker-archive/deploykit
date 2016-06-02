package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
)

// Credentials manages the objects used to authenticate and authorize for a provisioner's API
type Credentials interface {
	// ListIds
	ListIds() ([]storage.CredentialsID, *Error)

	// Get returns a credential identified by key
	Get(id storage.CredentialsID) (api.Credential, *Error)

	// Deletes the credential identified by key
	Delete(id storage.CredentialsID) *Error

	// CreateCredential adds a new credential from the input reader.
	CreateCredential(id storage.CredentialsID, input io.Reader, codec *Codec) *Error

	// UpdateCredential updates an existing credential
	UpdateCredential(id storage.CredentialsID, input io.Reader, codec *Codec) *Error
}

type credentials struct {
	store        storage.Credentials
	provisioners *MachineProvisioners
}

// NewCredentials creates an instance of the manager given the backing store.
func NewCredentials(store storage.Credentials, provisioners *MachineProvisioners) Credentials {
	return &credentials{store: store, provisioners: provisioners}
}

func (c *credentials) newCredential(provisionerName string) (api.Credential, error) {
	if builder, has := c.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultCredential(), nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

func (c *credentials) unmarshal(codec *Codec, data []byte, cred api.Credential) error {
	return codec.unmarshal(data, cred)
}

func (c *credentials) marshal(codec *Codec, cred api.Credential) ([]byte, error) {
	return codec.marshal(cred)
}

func (c *credentials) ListIds() ([]storage.CredentialsID, *Error) {
	ids, err := c.store.List()
	if err != nil {
		return nil, UnknownError(err)
	}
	return ids, nil
}

func (c *credentials) Get(id storage.CredentialsID) (api.Credential, *Error) {
	detail, err := c.newCredential(id.Provisioner)
	if err != nil {
		return nil, &Error{ErrBadInput, err.Error()}
	}

	err = c.store.GetCredentials(id, detail)
	if err != nil {
		return nil, UnknownError(err)
	}
	return detail, nil
}

func (c *credentials) Delete(id storage.CredentialsID) *Error {
	err := c.store.Delete(id)
	// TODO(wfarner): Give better visibility from the store to disambiguate between failure cases.
	if err != nil {
		return UnknownError(err)
	}
	return nil
}

func (c *credentials) exists(id storage.CredentialsID) bool {
	err := c.store.GetCredentials(id, new(api.CredentialBase))
	return err == nil
}

// CreateCredential creates a new credential from the input reader.
func (c *credentials) CreateCredential(id storage.CredentialsID, input io.Reader, codec *Codec) *Error {
	if c.exists(id) {
		return &Error{ErrDuplicate, fmt.Sprintf("Credentials already exists: %v", id)}
	}

	creds, err := c.newCredential(id.Provisioner)
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	if err = c.unmarshal(codec, buff, creds); err != nil {
		return UnknownError(err)
	}
	if err = c.store.Save(id, creds); err != nil {
		return UnknownError(err)
	}
	return nil
}

func (c *credentials) UpdateCredential(id storage.CredentialsID, input io.Reader, codec *Codec) *Error {
	if !c.exists(id) {
		return &Error{ErrNotFound, fmt.Sprintf("Credential not found: %v", id)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	creds, err := c.newCredential(id.Provisioner)
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}

	if err = c.unmarshal(codec, buff, creds); err != nil {
		return UnknownError(err)
	}

	if err = c.store.Save(id, creds); err != nil {
		return UnknownError(err)
	}
	return nil
}
