package filestores

import (
	"github.com/docker/libmachete/storage"
	"path"
)

type credentials struct {
	sandbox Sandbox
}

func credentialsPath(id storage.CredentialsID) string {
	return path.Join(id.Provisioner, id.Name)
}

// NewCredentials creates a new credentials store within the provided sandbox.
func NewCredentials(sandbox Sandbox) storage.Credentials {
	return &credentials{sandbox: sandbox}
}

func (c credentials) Save(id storage.CredentialsID, credentialsData interface{}) error {
	return c.sandbox.marshalAndSave(credentialsPath(id), credentialsData)
}

func (c credentials) List() ([]storage.CredentialsID, error) {
	contents, err := c.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.CredentialsID{}
	for _, element := range contents {
		dir, file := dirAndFile(element)
		ids = append(ids, storage.CredentialsID{Provisioner: dir, Name: file})
	}
	return ids, nil
}

func (c credentials) GetCredentials(id storage.CredentialsID, credentialsData interface{}) error {
	return c.sandbox.readAndUnmarshal(credentialsPath(id), credentialsData)
}

func (c credentials) Delete(id storage.CredentialsID) error {
	return c.sandbox.remove(credentialsPath(id))
}
