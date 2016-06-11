package api

import (
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"github.com/docker/libmachete/storage/filestore"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package storage -destination ../mock/storage/kv_store.go github.com/docker/libmachete/storage KvStore

func requireSuccessfulRun(t *testing.T, hostName string, tasks []spi.Task) {
	events, err := runTasks(
		tasks,
		MachineRecord{MachineSummary: MachineSummary{MachineName: MachineID(hostName)}},
		new(spi.BaseMachineRequest),
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

func TestSSHKeyGenAndRemove(t *testing.T) {
	sshKeys := NewSSHKeys(filestore.NewFileStore(afero.NewMemMapFs(), "/"))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "test-host"

	requireSuccessfulRun(t, host, []spi.Task{SSHKeyGen{sshKeys}})

	// Key should have been created.
	data, err := sshKeys.GetEncodedPublicKey(SSHKeyID(host))
	require.NoError(t, err)
	require.NotEmpty(t, data)

	requireSuccessfulRun(t, host, []spi.Task{SSHKeyRemove{sshKeys}})

	// Key should have been removed.
	_, err = sshKeys.GetEncodedPublicKey(SSHKeyID(host))
	require.Error(t, err)
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
	require.Equal(t, []SSHKeyID{
		SSHKeyID("k1"),
		SSHKeyID("k2"),
		SSHKeyID("k3"),
		SSHKeyID("k4")},
		ids)
}
