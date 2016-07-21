package server

import (
	"errors"
	"github.com/docker/libmachete/client"
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestClientServerRelay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(backend), func(r *testflight.Requester) {
		client := client.NewInstanceProvisioner(r.Url(""))

		id := instance.ID("instance-1")

		provisionData := "provision request"
		backend.EXPECT().Provision(provisionData).Return(&id, nil)
		returnedID, err := client.Provision(provisionData)
		require.NoError(t, err)
		require.Equal(t, id, *returnedID)

		backend.EXPECT().Destroy(id).Return(nil)
		err = client.Destroy(id)
		require.NoError(t, err)

		ids := []instance.ID{id, instance.ID("instance-2")}

		group := instance.GroupID("group-1")
		backend.EXPECT().ListGroup(group).Return(ids, nil)
		returnedIDs, err := client.ListGroup(group)
		require.NoError(t, err)
		require.Equal(t, ids, returnedIDs)

		shellCode := "echo hello"
		cmdOutput := "hello"

		backend.EXPECT().ShellExec(id, shellCode).Return(&cmdOutput, nil)
		returnedOutput, err := client.ShellExec(id, shellCode)
		require.NoError(t, err)
		require.Equal(t, cmdOutput, *returnedOutput)

		// A shell command fails, and includes output.
		backendError := errors.New("something went wrong")
		backend.EXPECT().ShellExec(id, shellCode).Return(&cmdOutput, backendError)
		returnedOutput, err = client.ShellExec(id, shellCode)
		require.Equal(t, spi.NewError(spi.ErrUnknown, ""), err)
		require.Equal(t, cmdOutput, *returnedOutput)

		// A shell command fails, and includes no output.
		var noOutput *string
		backend.EXPECT().ShellExec(id, shellCode).Return(noOutput, backendError)
		returnedOutput, err = client.ShellExec(id, shellCode)
		require.Equal(t, spi.NewError(spi.ErrUnknown, backendError.Error()), err)
		require.Equal(t, noOutput, returnedOutput)
	})
}
