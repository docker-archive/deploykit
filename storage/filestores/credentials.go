package filestores

import (
	"github.com/docker/libmachete/storage"
)

type credentials struct {
	sandbox Sandbox
}

// NewCredentials creates a new credentials store within the provided sandbox.
func NewCredentials(sandbox Sandbox) storage.Credentials {
	return &credentials{sandbox: sandbox}
}

func (c credentials) Save(id storage.CredentialsID, credentialsData interface{}) error {
	return c.sandbox.marshalAndSave(string(id), credentialsData)
}

func (c credentials) List() ([]storage.CredentialsID, error) {
	contents, err := c.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.CredentialsID{}
	for _, element := range contents {
		ids = append(ids, storage.CredentialsID(element))
	}
	return ids, nil
}

func (c credentials) GetCredentials(id storage.CredentialsID, credentialsData interface{}) error {
	return c.sandbox.readAndUnmarshal(string(id), credentialsData)
}

func (c credentials) Delete(id storage.CredentialsID) error {
	return c.sandbox.remove(string(id))
}
