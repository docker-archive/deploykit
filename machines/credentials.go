package machines

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
)

type credentials struct {
	store        storage.KvStore
	provisioners *MachineProvisioners
}

// NewCredentials creates an instance of the manager given the backing store.
func NewCredentials(store storage.KvStore, provisioners *MachineProvisioners) api.Credentials {
	return &credentials{store: store, provisioners: provisioners}
}

func (c *credentials) newCredential(provisionerName string) (spi.Credential, error) {
	if builder, has := c.provisioners.GetBuilder(provisionerName); has {
		return builder.DefaultCredential(), nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

func (c *credentials) marshal(codec api.Codec, cred spi.Credential) ([]byte, error) {
	return codec.Marshal(cred)
}

func keyFromCredentialsID(id api.CredentialsID) storage.Key {
	return storage.Key{Path: []string{id.Provisioner, id.Name}}
}

func credentialsIDFromKey(key storage.Key) api.CredentialsID {
	key.RequirePathLength(2)
	return api.CredentialsID{Provisioner: key.Path[0], Name: key.Path[1]}
}

func (c *credentials) ListIds() ([]api.CredentialsID, *api.Error) {
	keys, err := c.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	ids := []api.CredentialsID{}
	for _, key := range keys {
		ids = append(ids, credentialsIDFromKey(key))
	}

	return ids, nil
}

func (c *credentials) Get(id api.CredentialsID) (spi.Credential, *api.Error) {
	data, err := c.store.Get(keyFromCredentialsID(id))
	if err != nil {
		return nil, api.UnknownError(err)
	}

	detail, err := c.newCredential(id.Provisioner)
	if err != nil {
		return nil, &api.Error{api.ErrBadInput, err.Error()}
	}

	err = json.Unmarshal(data, detail)
	if err != nil {
		return nil, &api.Error{api.ErrBadInput, err.Error()}
	}

	return detail, nil
}

func (c *credentials) Delete(id api.CredentialsID) *api.Error {
	err := c.store.Delete(keyFromCredentialsID(id))
	// TODO(wfarner): Give better visibility from the store to disambiguate between failure cases.
	if err != nil {
		return api.UnknownError(err)
	}
	return nil
}

func (c *credentials) exists(id api.CredentialsID) bool {
	_, err := c.store.Get(keyFromCredentialsID(id))
	return err == nil
}

func (c credentials) save(id api.CredentialsID, creds spi.Credential) *api.Error {
	stored, err := json.Marshal(creds)
	if err != nil {
		return api.UnknownError(err)
	}

	if err = c.store.Save(keyFromCredentialsID(id), stored); err != nil {
		return api.UnknownError(err)
	}
	return nil
}

// CreateCredential creates a new credential from the input reader.
func (c *credentials) CreateCredential(id api.CredentialsID, input io.Reader, codec api.Codec) *api.Error {
	if c.exists(id) {
		return &api.Error{api.ErrDuplicate, fmt.Sprintf("Credentials already exists: %v", id)}
	}

	creds, err := c.newCredential(id.Provisioner)
	if err != nil {
		return &api.Error{api.ErrNotFound, err.Error()}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return api.UnknownError(err)
	}

	if err = codec.Unmarshal(buff, creds); err != nil {
		return api.UnknownError(err)
	}

	return c.save(id, creds)
}

func (c *credentials) UpdateCredential(id api.CredentialsID, input io.Reader, codec api.Codec) *api.Error {
	if !c.exists(id) {
		return &api.Error{api.ErrNotFound, fmt.Sprintf("Credential not found: %v", id)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return api.UnknownError(err)
	}

	creds, err := c.newCredential(id.Provisioner)
	if err != nil {
		return &api.Error{api.ErrNotFound, err.Error()}
	}

	if err = codec.Unmarshal(buff, creds); err != nil {
		return api.UnknownError(err)
	}

	return c.save(id, creds)
}
