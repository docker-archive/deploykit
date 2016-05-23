package filestores

import (
	"fmt"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
	"path/filepath"
)

type keys struct {
	sandbox *sandbox
}

// NewKeys creates a new key store backed by the local file system.
func NewKeys(dir string) (storage.Keys, error) {
	sandbox, err := newSandbox(dir)
	if err != nil {
		return nil, err
	}

	return &keys{sandbox: sandbox}, nil
}

// Save saves the key by id
func (m keys) Save(id storage.KeyID, keyPair *ssh.KeyPair) error {
	err := m.sandbox.Mkdir(m.keyPath(id))
	if err != nil {
		return fmt.Errorf("Failed to create key directory: %s", err)
	}
	return keyPair.Write(m.privateKeyFullPath(id), m.publicKeyFullPath(id), m.sandbox.saveBytes)
}

func (m keys) List() ([]storage.KeyID, error) {
	contents, err := m.sandbox.List()
	if err != nil {
		return nil, err
	}
	ids := []storage.KeyID{}
	for _, element := range contents {
		ids = append(ids, storage.KeyID(element))
	}
	return ids, nil
}

func (m keys) GetPublicKey(id storage.KeyID) ([]byte, error) {
	return m.sandbox.readBytes(m.publicKeyFullPath(id))
}

func (m keys) Delete(id storage.KeyID) error {
	return m.sandbox.RemoveAll(m.keyPath(id))
}

func (m keys) keyPath(id storage.KeyID) string {
	return string(id)
}

func (m keys) privateKeyFullPath(id storage.KeyID) string {
	return filepath.Join(m.keyPath(id), "id_rsa")
}

func (m keys) publicKeyFullPath(id storage.KeyID) string {
	return m.privateKeyFullPath(id) + ".pub"
}
