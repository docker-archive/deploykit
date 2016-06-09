package libmachete

import (
	mock_api "github.com/docker/libmachete/mock/provisioners/api"
	mock_storage "github.com/docker/libmachete/mock/storage"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package storage -destination mock/storage/kv_store.go github.com/docker/libmachete/storage KvStore

func TestDefaultSSHKeyGenAndRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "test-host"

	provisioner := mock_api.NewMockProvisioner(ctrl)
	keystore := mock_api.NewMockKeyStore(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := new(api.BaseMachineRequest)
	record := new(MachineRecord)
	record.MachineName = MachineID(host)
	tasks := []api.Task{
		TaskSSHKeyGen,
		TaskSSHKeyRemove,
	}

	keystore.EXPECT().NewKeyPair(api.SSHKeyID(host)).Return(nil)
	keystore.EXPECT().Remove(api.SSHKeyID(host)).Return(nil)

	events, err := runTasks(
		provisioner,
		keystore,
		tasks,
		record,
		cred,
		request,
		func(r MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *MachineRecord, s api.MachineRequest) {
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
	require.Equal(t, []api.SSHKeyID{
		api.SSHKeyID("k1"),
		api.SSHKeyID("k2"),
		api.SSHKeyID("k3"),
		api.SSHKeyID("k4")},
		ids)
}
