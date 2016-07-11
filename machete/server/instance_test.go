package server

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/api"
	mock_instance "github.com/docker/libmachete/machete/mock/spi/instance"
	"github.com/docker/libmachete/machete/spi"
	"github.com/docker/libmachete/machete/spi/instance"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

var BadInputError = spi.Error{Code: api.ErrBadInput, Message: "Bad Input"}

func TestListGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		group := "worker-nodes"
		ids := []instance.ID{"a", "b", "c"}

		provisioner.EXPECT().ListGroup(instance.GroupID(group)).Return(ids, nil)

		response := r.Get(fmt.Sprintf("/instance/?group=%s", group))
		require.Equal(t, 200, response.StatusCode)
		result := []instance.ID{}
		require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
		require.Equal(t, ids, result)
	})
}

func expectBadInputError(t *testing.T, response *testflight.Response) {
	require.Equal(t, 400, response.StatusCode)
	body := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(response.Body), &body))
	require.Equal(t, map[string]string{"error": BadInputError.Message}, body)
}

func TestListGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		// A group filter must be included.
		response := r.Get("/instance/")
		require.Equal(t, 400, response.StatusCode)

		group := "worker-nodes"
		provisioner.EXPECT().ListGroup(instance.GroupID(group)).Return(nil, &BadInputError)
		expectBadInputError(t, r.Get(fmt.Sprintf("/instance/?group=%s", group)))
	})
}

func TestProvision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		id := instance.ID("instance-id")
		request := "{}"

		provisioner.EXPECT().Provision(request).Return(&id, nil)

		response := r.Post("/instance/", "application/json", request)
		require.Equal(t, 201, response.StatusCode)
		var result instance.ID
		require.NoError(t, json.Unmarshal([]byte(response.Body), &result))
		require.Equal(t, id, result)
	})
}

func TestProvisionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		request := "{}"
		provisioner.EXPECT().Provision(request).Return(nil, &BadInputError)
		expectBadInputError(t, r.Post("/instance/", "application/json", request))
	})
}

func TestDestroy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)

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

	provisioner := mock_instance.NewMockProvisioner(ctrl)

	testflight.WithServer(NewHandler(provisioner), func(r *testflight.Requester) {
		id := instance.ID("instance-id")
		provisioner.EXPECT().Destroy(id).Return(&BadInputError)
		expectBadInputError(t, r.Delete(fmt.Sprintf("/instance/%s", id), "application/json", ""))
	})
}
