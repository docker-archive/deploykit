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
	ListIds() ([]string, error)

	// Saves the credential identified by key
	Save(key string, cred api.Credential) error

	// Get returns a credential identified by key
	Get(key string) (api.Credential, error)

	// Deletes the credential identified by key
	Delete(key string) error

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

func ensureValidContentType(ct *Codec) *Codec {
	if ct != nil {
		return ct
	}
	return DefaultContentType
}

func (c *credentials) newCredential(provisionerName string) (api.Credential, error) {
	if builder, has := c.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultCredential, nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

func (c *credentials) unmarshal(contentType *Codec, data []byte, cred api.Credential) error {
	return ensureValidContentType(contentType).unmarshal(data, cred)
}

func (c *credentials) marshal(contentType *Codec, cred api.Credential) ([]byte, error) {
	return ensureValidContentType(contentType).marshal(cred)
}

func (c *credentials) ListIds() ([]string, error) {
	out := []string{}
	list, err := c.store.List()
	if err != nil {
		return nil, err
	}
	for _, i := range list {
		out = append(out, string(i))
	}
	return out, nil
}

func (c *credentials) Save(key string, cred api.Credential) error {
	return c.store.Save(storage.CredentialsID(key), cred)
}

func (c *credentials) Get(key string) (api.Credential, error) {
	// Since we don't know the provider, we need to read twice: first with a base
	// structure, then with a specific structure by provisioner.
	base := new(api.CredentialBase)
	err := c.store.GetCredentials(storage.CredentialsID(key), base)
	if err != nil {
		return nil, err
	}

	detail, err := c.newCredential(base.ProvisionerName())
	if err != nil {
		return nil, err
	}

	err = c.store.GetCredentials(storage.CredentialsID(key), detail)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (c *credentials) Delete(key string) error {
	return c.store.Delete(storage.CredentialsID(key))
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

	cr, err := c.newCredential(provisioner)
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknown provisioner:%s", provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}
	}

	if err = c.unmarshal(codec, buff, cr); err != nil {
		return &Error{Message: err.Error()}
	}
	if err = c.Save(key, cr); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

func (c *credentials) UpdateCredential(key string, input io.Reader, codec *Codec) *Error {
	if !c.exists(key) {
		return &Error{ErrNotFound, fmt.Sprintf("Credential not found: %v", key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &Error{Message: err.Error()}

	}

	base := new(api.CredentialBase)
	if err = c.unmarshal(codec, buff, base); err != nil {
		return &Error{Message: err.Error()}
	}

	detail, err := c.newCredential(base.ProvisionerName())
	if err != nil {
		return &Error{ErrNotFound, fmt.Sprintf("Unknow provisioner: %v", base.ProvisionerName())}
	}

	if err = c.unmarshal(codec, buff, detail); err != nil {
		return &Error{Message: err.Error()}
	}

	if err = c.Save(key, detail); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}
