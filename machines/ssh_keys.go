package machines

import (
	"encoding/json"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/ssh"
	"github.com/docker/libmachete/storage"
)

type sshKeys struct {
	store storage.KvStore
}

// NewSSHKeys creates an instance of key manager
func NewSSHKeys(store storage.KvStore) api.SSHKeys {
	return &sshKeys{store: store}
}

func sshKeyIDFromKey(key storage.Key) api.SSHKeyID {
	key.RequirePathLength(1)
	return api.SSHKeyID(key.Path[0])
}

// TODO(wfarner): This should be a method on api.SSHKeyID, but is currently declared here to work around an import
// cycle.
func sshKeyIDToKey(sshID api.SSHKeyID) storage.Key {
	return storage.Key{Path: []string{string(sshID)}}
}

// NewKeyPair creates and saves a key pair
func (km *sshKeys) NewKeyPair(id api.SSHKeyID) error {
	if km.exists(id) {
		return api.NewError(api.ErrDuplicate, "Duplicate key: %v", id)
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
func (km *sshKeys) ListIds() ([]api.SSHKeyID, error) {
	keys, err := km.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, err
	}

	storage.SortKeys(keys)

	ids := []api.SSHKeyID{}
	for _, key := range keys {
		ids = append(ids, sshKeyIDFromKey(key))
	}
	return ids, nil
}

// GetEncodedPublicKey returns an OpenSSH authorized_key format public key
func (km *sshKeys) GetEncodedPublicKey(id api.SSHKeyID) ([]byte, error) {
	data, err := km.store.Get(sshKeyIDToKey(id))
	if err != nil {
		return nil, err
	}

	keyPair := ssh.KeyPair{}
	err = json.Unmarshal(data, &keyPair)
	return keyPair.EncodedPublicKey, err
}

// Exists returns true if machine identified by key already exists
func (km *sshKeys) exists(id api.SSHKeyID) bool {
	_, err := km.store.Get(sshKeyIDToKey(id))
	return err == nil
}

// Remove removes the key pair
func (km *sshKeys) Remove(id api.SSHKeyID) error {
	return km.store.Delete(sshKeyIDToKey(id))
}
