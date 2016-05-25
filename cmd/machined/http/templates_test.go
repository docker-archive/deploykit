package http

import (
	"encoding/json"
	"github.com/docker/libmachete"
	mock_api "github.com/docker/libmachete/mock/provisioners/api"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"github.com/docker/libmachete/storage/filestores"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func prepareTest(t *testing.T, ctrl *gomock.Controller) (*mock_api.MockProvisioner, http.Handler) {
	provisioner := mock_api.NewMockProvisioner(ctrl)

	builder := libmachete.ProvisionerBuilder{
		Name:                  "testcloud",
		DefaultCredential:     nil,
		DefaultMachineRequest: func() api.MachineRequest { return &api.BaseMachineRequest{} },
		Build: func(controls api.ProvisionControls, cred api.Credential) (api.Provisioner, error) {
			return provisioner, nil
		},
	}

	sandbox := filestores.NewSandbox(afero.NewMemMapFs(), "/")

	server, err := build(sandbox, libmachete.NewMachineProvisioners([]libmachete.ProvisionerBuilder{builder}))
	require.NoError(t, err)

	return provisioner, server.getHandler()
}

const JSON = "application/json"

func marshalTemplate(t *testing.T, template api.BaseMachineRequest) string {
	body, err := json.Marshal(template)
	require.NoError(t, err)
	return string(body)
}

func unmarshalTemplate(t *testing.T, data string) api.BaseMachineRequest {
	value := api.BaseMachineRequest{}
	err := json.Unmarshal([]byte(data), &value)
	require.NoError(t, err)
	return value
}

type ListResponse []storage.TemplateID

func requireListResult(t *testing.T, r *testflight.Requester, expected ListResponse) {
	response := r.Get("/templates/json")
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))
	payload := ListResponse{}
	require.NoError(t, json.Unmarshal([]byte(response.Body), &payload))
	require.Equal(t, expected, payload)
}

func TestTemplateCrud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// There should initially be no templates
		requireListResult(t, r, ListResponse{})

		// Create a template
		template := api.BaseMachineRequest{MachineName: "test", Provisioner: "testcloud"}
		response := r.Post("/templates/testcloud/prodtemplate/create", JSON, marshalTemplate(t, template))
		require.Equal(t, 200, response.StatusCode)

		// It should return by id
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		require.Equal(t, template, unmarshalTemplate(t, response.Body))

		id := storage.TemplateID{Provisioner: "testcloud", Name: "prodtemplate"}

		// It should appear in a list request
		requireListResult(t, r, ListResponse{id})

		// Update the template
		updated := api.BaseMachineRequest{MachineName: "testnew", Provisioner: "testcloud"}
		response = r.Put("/templates/testcloud/prodtemplate", JSON, marshalTemplate(t, updated))
		require.Equal(t, 200, response.StatusCode)

		// It should be updated
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		require.Equal(t, updated, unmarshalTemplate(t, response.Body))

		// It should still appear in a list request
		requireListResult(t, r, ListResponse{id})

		// Delete the template
		require.Equal(t, 200, r.Delete("/templates/testcloud/prodtemplate", JSON, "").StatusCode)

		// It should no longer exist
		require.Equal(t, 404, r.Get("/templates/testcloud/prodtemplate/json").StatusCode)
		requireListResult(t, r, ListResponse{})
	})
}
