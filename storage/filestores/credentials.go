package filestores

import (
	"github.com/docker/libmachete/storage"
)

type credentials struct {
	sandbox *sandbox
}

// NewCredentials creates a new credentials store backed by the local file system.
func NewCredentials(dir string) (storage.Credentials, error) {
	sandbox, err := newSandbox(dir)
	if err != nil {
		return nil, err
	}

	return &credentials{sandbox: sandbox}, nil
}

func (c credentials) Save(id storage.CredentialsID, credentialsData interface{}) error {
	return c.sandbox.MarshalAndSave(string(id), credentialsData)
}

func (c credentials) List() ([]storage.CredentialsID, error) {
	contents, err := c.sandbox.List()
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
	return c.sandbox.ReadAndUnmarshal(string(id), credentialsData)
}

func (c credentials) Delete(id storage.CredentialsID) error {
	return c.sandbox.Remove(string(id))
}
