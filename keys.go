package libmachete

import (
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
	"sort"
)

// Keys manages SSH keys
type Keys interface {

	// Includes basic key access and generation functionality
	api.KeyStore

	// ListIds
	ListIds() ([]string, error)
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

	if km.exists(id) {
		return NewError(ErrNotFound, "Key %v not found.", id)
	}

	return km.store.Save(storage.KeyID(id), kp)
}

// ListIds returns a list of key ids.
func (km *keys) ListIds() ([]string, error) {
	all, err := km.store.List()
	if err != nil {
		return nil, err
	}
	list := []string{}
	for _, k := range all {
		list = append(list, string(k))
	}
	sort.Strings(list)
	return list, nil
}

// GetEncodedPublicKey returns an OpenSSH authorized_key format public key
func (km *keys) GetEncodedPublicKey(id string) ([]byte, error) {
	return km.store.GetEncodedPublicKey(storage.KeyID(id))
}

// Exists returns true if machine identified by key already exists
func (km *keys) exists(id string) bool {
	_, err := km.store.GetEncodedPublicKey(storage.KeyID(id))
	return err == nil
}

// Remove removes the key pair
func (km *keys) Remove(id string) error {
	return km.store.Delete(storage.KeyID(id))
}
