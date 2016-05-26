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
	ListIds() ([]string, *Error)

	// Get returns a credential identified by key
	Get(key string) (api.Credential, *Error)

	// Deletes the credential identified by key
	Delete(key string) *Error

	// CreateCredential adds a new credential from the input reader.
	CreateCredential(provisioner, key string, input io.Reader, codec *Codec) *Error

	// UpdateCredential updates an existing credential
	UpdateCredential(key string, input io.Reader, codec *Codec) *Error
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

func (c *credentials) ListIds() ([]string, *Error) {
	out := []string{}
	list, err := c.store.List()
	if err != nil {
		return nil, UnknownError(err)
	}
	for _, i := range list {
		out = append(out, string(i))
	}
	return out, nil
}

func (c *credentials) Get(key string) (api.Credential, *Error) {
	// TODO(wfarner): Adjust this once credentials are scoped by provisioner.
	// Since we don't know the provisioner, we need to read twice: first with a base
	// structure, then with a specific structure by provisioner.
	base := new(api.CredentialBase)
	err := c.store.GetCredentials(storage.CredentialsID(key), base)
	if err != nil {
		return nil, &Error{ErrNotFound, err.Error()}
	}

	detail, err := c.newCredential(base.ProvisionerName())
	if err != nil {
		return nil, &Error{ErrBadInput, err.Error()}
	}

	err = c.store.GetCredentials(storage.CredentialsID(key), detail)
	if err != nil {
		return nil, UnknownError(err)
	}
	return detail, nil
}

func (c *credentials) Delete(key string) *Error {
	err := c.store.Delete(storage.CredentialsID(key))
	// TODO(wfarner): Give better visibility from the store to disambiguate between failure cases.
	if err != nil {
		return UnknownError(err)
	}
	return nil
}

func (c *credentials) exists(key string) bool {
	base := new(api.CredentialBase)
	err := c.store.GetCredentials(storage.CredentialsID(key), base)
	return err == nil
}

// CreateCredential creates a new credential from the input reader.
func (c *credentials) CreateCredential(provisioner, key string, input io.Reader, codec *Codec) *Error {
	if c.exists(key) {
		return &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	creds, err := c.newCredential(provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknown provisioner:%s", provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	if err = c.unmarshal(codec, buff, creds); err != nil {
		return UnknownError(err)
	}
	if err = c.store.Save(storage.CredentialsID(key), creds); err != nil {
		return UnknownError(err)
	}
	return nil
}

func (c *credentials) UpdateCredential(key string, input io.Reader, codec *Codec) *Error {
	if !c.exists(key) {
		return &Error{ErrNotFound, fmt.Sprintf("Credential not found: %v", key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	base := new(api.CredentialBase)
	if err = c.unmarshal(codec, buff, base); err != nil {
		return UnknownError(err)
	}

	creds, err := c.newCredential(base.ProvisionerName())
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}

	if err = c.unmarshal(codec, buff, creds); err != nil {
		return UnknownError(err)
	}

	if err = c.store.Save(storage.CredentialsID(key), creds); err != nil {
		return UnknownError(err)
	}
	return nil
}
