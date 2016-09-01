package server

import (
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

		provisionData := "{}"
		volume := instance.VolumeID("volume")
		backend.EXPECT().Provision(provisionData, &volume).Return(&id, nil)
		returnedID, err := client.Provision(provisionData, &volume)
		require.NoError(t, err)
		require.Equal(t, id, *returnedID)

		backend.EXPECT().Destroy(id).Return(nil)
		err = client.Destroy(id)
		require.NoError(t, err)

		descriptions := []instance.Description{
			{ID: id, PrivateIPAddress: "10.0.0.2"},
			{ID: instance.ID("instance-2"), PrivateIPAddress: "10.0.0.3"}}

		group := instance.GroupID("group-1")
		backend.EXPECT().DescribeInstances(group).Return(descriptions, nil)
		returnedDescriptions, err := client.DescribeInstances(group)
		require.NoError(t, err)
		require.Equal(t, descriptions, returnedDescriptions)
	})
}

func TestErrorMapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(backend), func(r *testflight.Requester) {
		frontend := client.NewInstanceProvisioner(r.Url(""))
		backendErr := spi.NewError(spi.ErrBadInput, "Bad")

		backend.EXPECT().Provision("{}", nil).Return(nil, backendErr)
		id, err := frontend.Provision("{}", nil)
		require.Equal(t, backendErr, err)
		require.Nil(t, id)
	})
}
