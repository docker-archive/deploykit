package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
)

// Keys manages SSH keys
type Keys interface {

	// Includes basic key access and generation functionality
	api.KeyStore

	// ListIds
	ListIds() ([]storage.KeyID, error)
}

type keys struct {
	store storage.Keys
}

// NewKeys creates an instance of key manager
func NewKeys(store storage.Keys) Keys {
	return &keys{store: store}
}

// NewKeyPair creates and saves a key pair
func (km *keys) NewKeyPair(id string) error {
	kp, err := ssh.NewKeyPair()
	if err != nil {
		return err
	}
	return km.store.Save(storage.KeyID(id), kp)
}

// ListIds returns a list of key ids.
func (km *keys) ListIds() ([]storage.KeyID, error) {
	return km.store.List()
}

// Get returns a public key
func (km *keys) GetPublicKey(id string) ([]byte, error) {
	return km.store.GetPublicKey(storage.KeyID(id))
}

// Exists returns true if machine identified by key already exists
func (km *keys) Exists(id string) bool {
	_, err := km.store.GetPublicKey(storage.KeyID(id))
	return err == nil
}

// Remove removes the key pair
func (km *keys) Remove(id string) error {
	return km.store.Delete(storage.KeyID(id))
}
