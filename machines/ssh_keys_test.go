package machines

import (
	"github.com/docker/libmachete/api"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package storage -destination ../mock/storage/kv_store.go github.com/docker/libmachete/storage KvStore

func storageKey(value string) storage.Key {
	return storage.Key{Path: []string{value}}
}

func TestKeysAreSorted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_storage.NewMockKvStore(ctrl)
	keys := NewSSHKeys(store)

	store.EXPECT().ListRecursive(storage.RootKey).Return(
		[]storage.Key{
			storageKey("k1"),
			storageKey("k2"),
			storageKey("k3"),
			storageKey("k4")},
		nil)

	ids, err := keys.ListIds()
	require.NoError(t, err)
	require.Equal(t, []api.SSHKeyID{
		api.SSHKeyID("k1"),
		api.SSHKeyID("k2"),
		api.SSHKeyID("k3"),
		api.SSHKeyID("k4")},
		ids)
}
