package api

import (
	"encoding/json"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
)

// SSHKeys provides operations for generating and managing SSH keys.
type SSHKeys interface {

	// NewKeyPair creates and saves a new key pair identified by the id
	NewKeyPair(id SSHKeyID) error

	// GetEncodedPublicKey returns the public key bytes for the key pair identified by id.
	// The format is in the OpenSSH authorized_keys format.
	GetEncodedPublicKey(id SSHKeyID) ([]byte, error)

	// Remove the keypair
	Remove(id SSHKeyID) error

	// ListIds
	ListIds() ([]SSHKeyID, error)
}

// SSHKeyID is a unique id for an SSH key
type SSHKeyID string

type sshKeys struct {
	store storage.KvStore
}

// NewSSHKeys creates an instance of key manager
func NewSSHKeys(store storage.KvStore) SSHKeys {
	return &sshKeys{store: store}
}

func sshKeyIDFromKey(key storage.Key) SSHKeyID {
	requirePathLength(key, 1)
	return SSHKeyID(key.Path[0])
}

// TODO(wfarner): This should be a method on SSHKeyID, but is currently declared here to work around an import
// cycle.
func sshKeyIDToKey(sshID SSHKeyID) storage.Key {
	return storage.Key{Path: []string{string(sshID)}}
}

// NewKeyPair creates and saves a key pair
func (km *sshKeys) NewKeyPair(id SSHKeyID) error {
	if km.exists(id) {
		return NewError(ErrDuplicate, "Duplicate key: %v", id)
	}

	keyPair, err := ssh.NewKeyPair()
	if err != nil {
		return err
	}

	data, err := json.Marshal(keyPair)
	if err != nil {
		return err
	}

	return km.store.Save(sshKeyIDToKey(id), data)
}

// ListIds returns a list of key ids.
func (km *sshKeys) ListIds() ([]SSHKeyID, error) {
	keys, err := km.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, err
	}

	storage.SortKeys(keys)

	ids := []SSHKeyID{}
	for _, key := range keys {
		ids = append(ids, sshKeyIDFromKey(key))
	}
	return ids, nil
}

// GetEncodedPublicKey returns an OpenSSH authorized_key format public key
func (km *sshKeys) GetEncodedPublicKey(id SSHKeyID) ([]byte, error) {
	data, err := km.store.Get(sshKeyIDToKey(id))
	if err != nil {
		return nil, err
	}

	keyPair := ssh.KeyPair{}
	err = json.Unmarshal(data, &keyPair)
	return keyPair.EncodedPublicKey, err
}

// Exists returns true if machine identified by key already exists
func (km *sshKeys) exists(id SSHKeyID) bool {
	_, err := km.store.Get(sshKeyIDToKey(id))
	return err == nil
}

// Remove removes the key pair
func (km *sshKeys) Remove(id SSHKeyID) error {
	return km.store.Delete(sshKeyIDToKey(id))
}
