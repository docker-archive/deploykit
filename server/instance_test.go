package server

import (
	"encoding/json"
	"fmt"
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/server/api"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

var BadInputError = spi.NewError(spi.ErrBadInput, "Bad Input")

func TestListGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		gid := group.ID("worker-nodes")
		descriptions := []instance.Description{
			{ID: instance.ID("a"), PrivateIPAddress: "10.0.0.3"},
			{ID: instance.ID("b"), PrivateIPAddress: "10.0.0.4"},
			{ID: instance.ID("c"), PrivateIPAddress: "10.0.0.5"},
		}

		provisioner.EXPECT().DescribeInstances(gid).Return(descriptions, nil)

		response := r.Get(fmt.Sprintf("/instance/?group=%s", gid))
		require.Equal(t, 200, response.StatusCode)
		result := []instance.Description{}
		require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
		require.Equal(t, descriptions, result)
	})
}

func expectBadInputError(t *testing.T, response *testflight.Response) {
	require.Equal(t, 400, response.StatusCode)
	body := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(response.Body), &body))
	require.Equal(t, map[string]string{"error": BadInputError.Error()}, body)
}

func TestListGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		// A group filter must be included.
		response := r.Get("/instance/")
		require.Equal(t, 400, response.StatusCode)

		gid := group.ID("worker-nodes")
		provisioner.EXPECT().DescribeInstances(gid).Return(nil, BadInputError)
		expectBadInputError(t, r.Get(fmt.Sprintf("/instance/?group=%s", gid)))
	})
}

func TestProvision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		gid := group.ID("workers")
		id := instance.ID("instance-id")

		request := "{}"
		volume := instance.VolumeID("volume")

		provisioner.EXPECT().Provision(gid, request, &volume).Return(&id, nil)

		payload := json.RawMessage(request)
		body, err := json.Marshal(api.ProvisionRequest{
			Group:   gid,
			Request: &payload,
			Volume:  &volume,
		})
		require.NoError(t, err)

		response := r.Post("/instance/", "application/json", string(body))
		require.Equal(t, 201, response.StatusCode)
		var result instance.ID
		require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
		require.Equal(t, id, result)
	})
}

func TestProvisionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		gid := group.ID("workers")
		request := "{}"
		provisioner.EXPECT().Provision(gid, request, nil).Return(nil, BadInputError)

		payload := json.RawMessage(request)
		body, err := json.Marshal(api.ProvisionRequest{Group: gid, Request: &payload})
		require.NoError(t, err)

		expectBadInputError(t, r.Post("/instance/", "application/json", string(body)))
	})
}

func TestDestroy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		id := instance.ID("instance-id")
		provisioner.EXPECT().Destroy(id).Return(nil)

		response := r.Delete(fmt.Sprintf("/instance/%s", id), "application/json", "")
		require.Equal(t, 200, response.StatusCode)
	})
}

func TestDestroyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		id := instance.ID("instance-id")
		provisioner.EXPECT().Destroy(id).Return(BadInputError)
		expectBadInputError(t, r.Delete(fmt.Sprintf("/instance/%s", id), "application/json", ""))
	})
}
