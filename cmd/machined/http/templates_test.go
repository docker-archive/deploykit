package http

import (
	"encoding/json"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

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

type TemplateList []libmachete.TemplateID

func requireTemplates(t *testing.T, r *testflight.Requester, expected TemplateList) {
	response := r.Get("/templates/json")
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))
	payload := TemplateList{}
	require.NoError(t, json.Unmarshal([]byte(response.Body), &payload))
	require.Equal(t, expected, payload)
}

func TestTemplateCrud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// There should initially be no templates
		requireTemplates(t, r, TemplateList{})

		// Create a template
		template := api.BaseMachineRequest{MachineName: "test", Provisioner: "testcloud"}
		response := r.Post("/templates/testcloud/prodtemplate/create", JSON, marshalTemplate(t, template))
		require.Equal(t, 200, response.StatusCode)

		// It should return by id
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		require.Equal(t, template, unmarshalTemplate(t, response.Body))

		id := libmachete.TemplateID{Provisioner: "testcloud", Name: "prodtemplate"}

		// It should appear in a list request
		requireTemplates(t, r, TemplateList{id})

		// Update the template
		updated := api.BaseMachineRequest{MachineName: "testnew", Provisioner: "testcloud"}
		response = r.Put("/templates/testcloud/prodtemplate", JSON, marshalTemplate(t, updated))
		require.Equal(t, 200, response.StatusCode)

		// It should be updated
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		require.Equal(t, updated, unmarshalTemplate(t, response.Body))

		// It should still appear in a list request
		requireTemplates(t, r, TemplateList{id})

		// Delete the template
		require.Equal(t, 200, r.Delete("/templates/testcloud/prodtemplate", JSON, "").StatusCode)

		// It should no longer exist
		require.Equal(t, 404, r.Get("/templates/testcloud/prodtemplate/json").StatusCode)
		requireTemplates(t, r, TemplateList{})
	})
}

func TestErrorResponses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		template := api.BaseMachineRequest{MachineName: "test", Provisioner: "testcloud"}

		// Non-existent provisioner and/or template.
		require.Equal(t, 400, r.Get("/templates/meta/absentprovisioner/json").StatusCode)
		require.Equal(t, 400, r.Post(
			"/templates/absentprovisioner/name/create",
			JSON,
			marshalTemplate(t, template)).StatusCode)
		require.Equal(t, 404, r.Put(
			"/templates/absentprovisioner/name",
			JSON,
			marshalTemplate(t, template)).StatusCode)
		require.Equal(t, 404, r.Get("/templates/absentprovisioner/name").StatusCode)
		require.Equal(t, 404, r.Delete("/templates/absentprovisioner/name", JSON, "").StatusCode)

		// Duplicate template.
		response := r.Post("/templates/testcloud/prodtemplate/create", JSON, marshalTemplate(t, template))
		require.Equal(t, 200, response.StatusCode)
		response = r.Post("/templates/testcloud/prodtemplate/create", JSON, marshalTemplate(t, template))
		require.Equal(t, 409, response.StatusCode)
	})
}
