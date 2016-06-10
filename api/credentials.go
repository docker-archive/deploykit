package api

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
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
	CreateCredential(id CredentialsID, input io.Reader, codec *Codec) *Error

	// UpdateCredential updates an existing credential
	UpdateCredential(id CredentialsID, input io.Reader, codec *Codec) *Error
}

type credentials struct {
	store        storage.KvStore
	provisioners *MachineProvisioners
}

// NewCredentials creates an instance of the manager given the backing store.
func NewCredentials(store storage.KvStore, provisioners *MachineProvisioners) Credentials {
	return &credentials{store: store, provisioners: provisioners}
}

func (c *credentials) newCredential(provisionerName string) (spi.Credential, error) {
	if builder, has := c.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultCredential(), nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

func (c *credentials) unmarshal(codec *Codec, data []byte, cred spi.Credential) error {
	return codec.unmarshal(data, cred)
}

func (c *credentials) marshal(codec *Codec, cred spi.Credential) ([]byte, error) {
	return codec.marshal(cred)
}

func keyFromCredentialsID(id CredentialsID) storage.Key {
	return storage.Key{Path: []string{id.Provisioner, id.Name}}
}

func credentialsIDFromKey(key storage.Key) CredentialsID {
	requirePathLength(key, 2)
	return CredentialsID{Provisioner: key.Path[0], Name: key.Path[1]}
}

func (c *credentials) ListIds() ([]CredentialsID, *Error) {
	keys, err := c.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, UnknownError(err)
	}

	ids := []CredentialsID{}
	for _, key := range keys {
		ids = append(ids, credentialsIDFromKey(key))
	}

	return ids, nil
}

func (c *credentials) Get(id CredentialsID) (spi.Credential, *Error) {
	data, err := c.store.Get(keyFromCredentialsID(id))
	if err != nil {
		return nil, &Error{Code: ErrNotFound, Message: "Credentials entry does not exist"}
	}

	detail, err := c.newCredential(id.Provisioner)
	if err != nil {
		return nil, &Error{ErrBadInput, err.Error()}
	}

	err = json.Unmarshal(data, detail)
	if err != nil {
		return nil, &Error{ErrBadInput, err.Error()}
	}

	return detail, nil
}

func (c *credentials) Delete(id CredentialsID) *Error {
	err := c.store.Delete(keyFromCredentialsID(id))
	if err != nil {
		return &Error{ErrNotFound, err.Error()}
	}
	return nil
}

func (c *credentials) exists(id CredentialsID) bool {
	_, err := c.store.Get(keyFromCredentialsID(id))
	return err == nil
}

func (c credentials) save(id CredentialsID, creds spi.Credential) *Error {
	stored, err := json.Marshal(creds)
	if err != nil {
		return UnknownError(err)
	}

	if err = c.store.Save(keyFromCredentialsID(id), stored); err != nil {
		return UnknownError(err)
	}
	return nil
}

// CreateCredential creates a new credential from the input reader.
func (c *credentials) CreateCredential(id CredentialsID, input io.Reader, codec *Codec) *Error {
	if c.exists(id) {
		return &Error{ErrDuplicate, fmt.Sprintf("Credentials already exists: %v", id)}
	}

	creds, err := c.newCredential(id.Provisioner)
	if err != nil {
		return &Error{ErrBadInput, err.Error()}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return UnknownError(err)
	}

	if err = c.unmarshal(codec, buff, creds); err != nil {
		return UnknownError(err)
	}

	return c.save(id, creds)
}

func (c *credentials) UpdateCredential(id CredentialsID, input io.Reader, codec *Codec) *Error {
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

	return c.save(id, creds)
}
