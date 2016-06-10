package http

import (
	"github.com/docker/libmachete/api"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type CredentialsList []api.CredentialsID

func requireCredentials(t *testing.T, r *testflight.Requester, expected CredentialsList) {
	response := r.Get("/credentials/json")
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))
	requireUnmarshalEqual(t, &expected, response.Body, &CredentialsList{})
}

func TestCredentialsCrud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// There should initially be no credentials
		requireCredentials(t, r, CredentialsList{})

		// Create an entry
		credentials := testCredentials{Identity: "larry", Secret: "12345"}
		response := r.Post(
			"/credentials/testcloud/production/create",
			JSON,
			requireMarshalSuccess(t, credentials))
		require.Equal(t, 200, response.StatusCode)

		// It should return by id
		response = r.Get("/credentials/testcloud/production/json")
		require.Equal(t, 200, response.StatusCode)
		requireUnmarshalEqual(t, &credentials, response.Body, &testCredentials{})

		id := api.CredentialsID{Provisioner: "testcloud", Name: "production"}

		// It should appear in a list request
		requireCredentials(t, r, CredentialsList{id})

		// Update the entry
		updated := testCredentials{Identity: "larry", Secret: "password"}
		response = r.Put("/credentials/testcloud/production", JSON, requireMarshalSuccess(t, updated))
		require.Equal(t, 200, response.StatusCode)

		// It should be updated
		response = r.Get("/credentials/testcloud/production/json")
		require.Equal(t, 200, response.StatusCode)
		requireUnmarshalEqual(t, &updated, response.Body, &testCredentials{})

		// It should still appear in a list request
		requireCredentials(t, r, CredentialsList{id})

		// Delete the entry
		require.Equal(t, 200, r.Delete("/credentials/testcloud/production", JSON, "").StatusCode)

		// It should no longer exist
		require.Equal(t, 404, r.Get("/credentials/testcloud/production/json").StatusCode)
		requireCredentials(t, r, CredentialsList{})
	})
}

func TestCredentialsErrorResponses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		credentials := testCredentials{Identity: "larry", Secret: "12345"}

		// Non-existent provisioner and/or credentials.
		require.Equal(t, 400, r.Post(
			"/credentials/absentprovisioner/name/create",
			JSON,
			requireMarshalSuccess(t, credentials)).StatusCode)
		require.Equal(t, 404, r.Put(
			"/credentials/absentprovisioner/name",
			JSON,
			requireMarshalSuccess(t, credentials)).StatusCode)
		require.Equal(t, 404, r.Get("/credentials/absentprovisioner/name").StatusCode)
		require.Equal(t, 404, r.Delete("/credentials/absentprovisioner/name", JSON, "").StatusCode)

		// Duplicate credentials.
		response := r.Post(
			"/credentials/testcloud/production/create",
			JSON,
			requireMarshalSuccess(t, credentials))
		require.Equal(t, 200, response.StatusCode)
		response = r.Post(
			"/credentials/testcloud/production/create",
			JSON,
			requireMarshalSuccess(t, credentials))
		require.Equal(t, 409, response.StatusCode)
	})
}
