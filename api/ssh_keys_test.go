package api

import (
	mock_spi "github.com/docker/libmachete/mock/provisioners/spi"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package storage -destination ../mock/storage/kv_store.go github.com/docker/libmachete/storage KvStore

func TestDefaultSSHKeyGenAndRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "test-host"

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	keystore := mock_spi.NewMockKeyStore(ctrl)
	cred := mock_spi.NewMockCredential(ctrl)
	request := new(spi.BaseMachineRequest)
	record := new(MachineRecord)
	record.MachineName = MachineID(host)
	tasks := []spi.Task{
		TaskSSHKeyGen,
		TaskSSHKeyRemove,
	}

	keystore.EXPECT().NewKeyPair(spi.SSHKeyID(host)).Return(nil)
	keystore.EXPECT().Remove(spi.SSHKeyID(host)).Return(nil)

	events, err := runTasks(
		provisioner,
		keystore,
		tasks,
		record,
		cred,
		request,
		func(r MachineRecord, q spi.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s spi.MachineRequest) {
			return
		})
	require.NoError(t, err)
	require.NotNil(t, events)
	checkTasksAreRun(t, events, tasks)
}

func storageKey(value string) storage.Key {
	return storage.Key{Path: []string{value}}
}

func TestSortedKeys(t *testing.T) {
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
	require.Equal(t, []spi.SSHKeyID{
		spi.SSHKeyID("k1"),
		spi.SSHKeyID("k2"),
		spi.SSHKeyID("k3"),
		spi.SSHKeyID("k4")},
		ids)
}
