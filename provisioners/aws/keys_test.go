package aws

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/machines"
	"github.com/docker/libmachete/provisioners/aws/mock"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage/filestore"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func newTestClientAndProvisioner(ctrl *gomock.Controller) (*mock.MockEC2API, provisioner) {
	client := mock.NewMockEC2API(ctrl)
	provisioner := provisioner{
		client:        client,
		sleepFunction: func(_ time.Duration) {},
		config:        defaultConfig(),
		sshKeys:       machines.NewSSHKeys(filestore.NewFileStore(afero.NewMemMapFs(), "/")),
	}
	return client, provisioner
}

func getTaskByName(t *testing.T, tasks []spi.Task, name string) spi.Task {
	for _, task := range tasks {
		if task.Name() == name {
			return task
		}
	}
	require.FailNow(t, "Task not found")
	return nil
}

func TestTaskGenerateAndUploadSSHKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "new-host"
	client, provisioner := newTestClientAndProvisioner(ctrl)

	matcher := func(input *ec2.ImportKeyPairInput) {
		require.NotNil(t, input)
		require.NotNil(t, input.KeyName)
		require.Equal(t, host, *input.KeyName)
		require.True(t, len(input.PublicKeyMaterial) > 0)
	}
	client.EXPECT().ImportKeyPair(gomock.Any()).Do(matcher).Return(nil, nil)

	request := &createInstanceRequest{}
	request.KeyName = ""
	request.MachineName = host

	events := make(chan interface{})
	record := &api.MachineRecord{}
	record.MachineName = api.MachineID(host)

	go func() {
		for range events {
			// consume events
		}
	}()

	// runs synchronously
	err := getTaskByName(t, provisioner.GetProvisionTasks(), machines.SSHKeyGenerateName).
		Run(record, request, events)

	close(events)

	require.NoError(t, err)

	// Verify that the keyName has been updated.  Note the line above the KeyName
	// was not specified.
	require.Equal(t, host, request.KeyName)
}

func TestTaskRemoveLocalAndUploadedSSHKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "new-host"
	keys := machines.NewSSHKeys(filestore.NewFileStore(afero.NewMemMapFs(), "/"))

	client, provisioner := newTestClientAndProvisioner(ctrl)

	client.EXPECT().DeleteKeyPair(&ec2.DeleteKeyPairInput{KeyName: &host}).Return(nil, nil)

	request := &createInstanceRequest{}
	require.NoError(t, json.Unmarshal([]byte(testCreateSync[0]), request))
	payload := fmt.Sprintf(`{"name": "%s", "provision" : [ "ssh-key-generate", "instance-create" ]}`, host)
	require.NoError(t, json.Unmarshal([]byte(payload), request))

	require.Equal(t, host, request.Name())

	events := make(chan interface{})
	record := &api.MachineRecord{}
	record.MachineName = api.MachineID(host)

	err := getTaskByName(t, provisioner.GetTeardownTasks(), machines.SSHKeyRemoveName).
		Run(record, request, events)
	// TODO(wfarner): This was previously require.NoError(), which does not seem to make sense since the SSH key
	// being deleted did not exist, which should be an error.  Consider adding a step to the beginning of the test
	// that first creates the SSH key.
	require.Error(t, err)

	close(events)
	ids, err := keys.ListIds()
	require.NoError(t, err)
	require.Equal(t, []api.SSHKeyID{}, ids)
}

func TestGeneratedKeyNameIsPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client, provisioner := newTestClientAndProvisioner(ctrl)

	host := "new-host"
	client.EXPECT().ImportKeyPair(gomock.Any()).Do(
		func(input *ec2.ImportKeyPairInput) {
			require.NotNil(t, input)
			require.NotNil(t, input.KeyName)
			require.Equal(t, host, *input.KeyName)
			require.True(t, len(input.PublicKeyMaterial) > 0)
		},
	).Return(nil, nil)

	events := make(chan interface{})
	record := api.MachineRecord{MachineSummary: api.MachineSummary{MachineName: api.MachineID(host)}}
	request := &createInstanceRequest{}

	go func() {
		err := getTaskByName(t, provisioner.GetProvisionTasks(), machines.SSHKeyGenerateName).
			Run(record, request, events)
		require.NoError(t, err)
		close(events)
	}()

	for event := range events {
		request, is := event.(*createInstanceRequest)
		require.True(t, is)
		require.Equal(t, string(record.MachineName), request.KeyName)
	}
}
