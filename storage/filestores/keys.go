package filestores

import (
	"fmt"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
	"path/filepath"
)

type keys struct {
	sandbox Sandbox
}

// NewKeys creates a new key store backed by the local file system.
func NewKeys(sandbox Sandbox) storage.Keys {
	return &keys{sandbox: sandbox}
}

// Save saves the key by id
func (m keys) Save(id storage.KeyID, keyPair *ssh.KeyPair) error {
	err := m.sandbox.mkdir(m.keyPath(id))
	if err != nil {
		return fmt.Errorf("Failed to create key directory: %s", err)
	}
	return m.sandbox.marshalAndSave(m.keyFullPath(id), keyPair)
}

func (m keys) List() ([]storage.KeyID, error) {
	contents, err := m.sandbox.list()
	if err != nil {
		return nil, err
	}
	ids := []storage.KeyID{}
	for _, element := range contents {
		ids = append(ids, storage.KeyID(element))
	}
	return ids, nil
}

// GetEncodedPublicKey returns the pem encoded public key as bytes
func (m keys) GetEncodedPublicKey(id storage.KeyID) ([]byte, error) {
	kp := new(ssh.KeyPair)
	err := m.sandbox.readAndUnmarshal(m.keyFullPath(id), kp)
	if err != nil {
		return nil, err
	}
	return kp.EncodedPublicKey, nil
}

func (m keys) Delete(id storage.KeyID) error {
	return m.sandbox.removeAll(m.keyPath(id))
}

func (m keys) keyPath(id storage.KeyID) string {
	return string(id)
}

func (m keys) keyFullPath(id storage.KeyID) string {
	return filepath.Join(m.keyPath(id), "key.json")
}
