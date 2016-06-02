package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete"
	mock_api "github.com/docker/libmachete/mock/provisioners/api"
	"github.com/docker/libmachete/provisioners/aws/mock"
	"github.com/docker/libmachete/storage"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTaskGenerateAndUploadSSHKey(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "new-host"
	sandbox := filestores.NewSandbox(afero.NewMemMapFs(), "/")
	keystore := filestores.NewKeys(sandbox)
	keys := libmachete.NewKeys(keystore)

	cred := mock_api.NewMockCredential(ctrl)
	client := mock.NewMockEC2API(ctrl)
	provisioner := New(client)

	matcher := func(input *ec2.ImportKeyPairInput) {
		require.NotNil(t, input)
		require.NotNil(t, input.KeyName)
		require.Equal(t, host, *input.KeyName)
		require.True(t, len(input.PublicKeyMaterial) > 0)
	}
	client.EXPECT().ImportKeyPair(gomock.Any()).Do(matcher).Return(nil, nil)

	request := new(CreateInstanceRequest)
	request.KeyName = ""
	request.MachineName = host

	events := make(chan interface{})
	record := &storage.MachineRecord{}
	record.MachineName = storage.MachineID(host)

	go func() {
		for range events {
			// consume events
		}
	}()

	// blocks synchronously
	err := GenerateAndUploadSSHKey(provisioner, keys, cred, record, request, events)

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
	sandbox := filestores.NewSandbox(afero.NewMemMapFs(), "/")
	keystore := filestores.NewKeys(sandbox)
	keys := libmachete.NewKeys(keystore)

	cred := mock_api.NewMockCredential(ctrl)
	client := mock.NewMockEC2API(ctrl)
	provisioner := New(client)

	client.EXPECT().DeleteKeyPair(&ec2.DeleteKeyPairInput{KeyName: &host}).Return(nil, nil)

	request := new(CreateInstanceRequest)
	require.NoError(t, json.Unmarshal([]byte(testCreateSync[0]), request))
	payload := fmt.Sprintf(`{"name": "%s", "provision" : [ "ssh-key-generate", "instance-create" ]}`, host)
	require.NoError(t, json.Unmarshal([]byte(payload), request))

	require.Equal(t, host, request.Name())

	events := make(chan interface{})
	record := &storage.MachineRecord{}
	record.MachineName = storage.MachineID(host)

	err := RemoveLocalAndUploadedSSHKey(provisioner, keys, cred, record, request, events)
	require.NoError(t, err)

	close(events)
	ids, err := keys.ListIds()
	require.NoError(t, err)
	require.Equal(t, []string{}, ids)
}

func TestGeneratedKeyNameIsPropagated(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	host := "new-host"
	sandbox := filestores.NewSandbox(afero.NewMemMapFs(), "/")
	machinestore := filestores.NewMachines(sandbox)
	keystore := filestores.NewKeys(sandbox)
	keys := libmachete.NewKeys(keystore)
	machines := libmachete.NewMachines(machinestore)

	cred := mock_api.NewMockCredential(ctrl)
	client := mock.NewMockEC2API(ctrl)
	provisioner := New(client)

	client.EXPECT().ImportKeyPair(gomock.Any()).Do(
		func(input *ec2.ImportKeyPairInput) {
			require.NotNil(t, input)
			require.NotNil(t, input.KeyName)
			require.Equal(t, host, *input.KeyName)
			require.True(t, len(input.PublicKeyMaterial) > 0)
		},
	).Return(nil, nil)
	client.EXPECT().RunInstances(gomock.Any()).Do(
		func(input *ec2.RunInstancesInput) {
			require.NotNil(t, input)
			require.Equal(t, host, *input.KeyName)
		},
	).Return(nil, nil)

	template := loadTemplate(t)

	events, err := machines.CreateMachine(
		provisioner,
		keys,
		cred,
		template,
		bytes.NewBuffer([]byte(fmt.Sprintf(`{"name": "%s", "provision" : [ "ssh-key-generate", "instance-create" ]}`, host))),
		libmachete.ContentTypeJSON)

	require.NoError(t, err)
	require.NotNil(t, events)

	for range events {
	}

	close(events)
}

func loadTemplate(t *testing.T) *CreateInstanceRequest {
	template := new(CreateInstanceRequest)
	require.NoError(t, json.Unmarshal([]byte(testCreateSync[0]), template))
	return template
}
