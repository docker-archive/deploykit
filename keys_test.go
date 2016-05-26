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

//go:generate mockgen -package storage -destination mock/storage/keys.go github.com/docker/libmachete/storage Keys

func TestDefaultSSHKeyGenAndRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "test-host"

	provisioner := mock_api.NewMockProvisioner(ctrl)
	keystore := mock_api.NewMockKeyStore(ctrl)
	cred := mock_api.NewMockCredential(ctrl)
	request := new(api.BaseMachineRequest)
	record := new(storage.MachineRecord)
	record.MachineName = storage.MachineID(host)
	tasks := []api.Task{
		TaskSSHKeyGen,
		TaskSSHKeyRemove,
	}

	keystore.EXPECT().NewKeyPair(host).Return(nil)
	keystore.EXPECT().Remove(host).Return(nil)

	events, err := runTasks(
		provisioner,
		keystore,
		tasks,
		record,
		cred,
		request,
		func(r storage.MachineRecord, q api.MachineRequest) error {
			return nil
		},
		func(r *storage.MachineRecord, s api.MachineRequest) {
			return
		})
	require.NoError(t, err)
	require.NotNil(t, events)
	checkTasksAreRun(t, events, tasks)
}

func TestSortedKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	keystore := mock_storage.NewMockKeys(ctrl)
	keys := NewKeys(keystore)

	keystore.EXPECT().List().Return([]storage.KeyID{
		storage.KeyID("k1"),
		storage.KeyID("k3"),
		storage.KeyID("k4"),
		storage.KeyID("k2"),
	}, nil)

	ids, err := keys.ListIds()
	require.NoError(t, err)
	require.Equal(t, []string{"k1", "k2", "k3", "k4"}, ids)
}
