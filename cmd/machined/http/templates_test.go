package http

import (
	"github.com/docker/libmachete/api"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/drewolson/testflight"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type TemplateList []api.TemplateID

func requireTemplates(t *testing.T, r *testflight.Requester, expected TemplateList) {
	response := r.Get("/templates/json")
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, JSON, response.Header.Get("Content-Type"))
	requireUnmarshalEqual(t, &expected, response.Body, &TemplateList{})
}

func TestTemplateCrud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		// There should initially be no templates
		requireTemplates(t, r, TemplateList{})

		// Create a template
		template := spi.BaseMachineRequest{MachineName: "test", Provisioner: "testcloud"}
		response := r.Post("/templates/testcloud/prodtemplate/create", JSON, requireMarshalSuccess(t, template))
		require.Equal(t, 200, response.StatusCode)

		// It should return by id
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		value := spi.BaseMachineRequest{}
		requireUnmarshalEqual(t, &template, response.Body, &value)

		id := api.TemplateID{Provisioner: "testcloud", Name: "prodtemplate"}

		// It should appear in a list request
		requireTemplates(t, r, TemplateList{id})

		// Update the template
		updated := spi.BaseMachineRequest{MachineName: "testnew", Provisioner: "testcloud"}
		response = r.Put("/templates/testcloud/prodtemplate", JSON, requireMarshalSuccess(t, updated))
		require.Equal(t, 200, response.StatusCode)

		// It should be updated
		response = r.Get("/templates/testcloud/prodtemplate/json")
		require.Equal(t, 200, response.StatusCode)
		value = spi.BaseMachineRequest{}
		requireUnmarshalEqual(t, &updated, response.Body, &value)

		// It should still appear in a list request
		requireTemplates(t, r, TemplateList{id})

		// Delete the template
		require.Equal(t, 200, r.Delete("/templates/testcloud/prodtemplate", JSON, "").StatusCode)

		// It should no longer exist
		require.Equal(t, 404, r.Get("/templates/testcloud/prodtemplate/json").StatusCode)
		requireTemplates(t, r, TemplateList{})
	})
}

func TestTemplatesErrorResponses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, handler := prepareTest(t, ctrl)

	testflight.WithServer(handler, func(r *testflight.Requester) {
		template := spi.BaseMachineRequest{MachineName: "test", Provisioner: "testcloud"}

		// Non-existent provisioner and/or template.
		require.Equal(t, 400, r.Get("/templates/meta/absentprovisioner/json").StatusCode)
		require.Equal(t, 400, r.Post(
			"/templates/absentprovisioner/name/create",
			JSON,
			requireMarshalSuccess(t, template)).StatusCode)
		require.Equal(t, 404, r.Put(
			"/templates/absentprovisioner/name",
			JSON,
			requireMarshalSuccess(t, template)).StatusCode)
		require.Equal(t, 404, r.Get("/templates/absentprovisioner/name").StatusCode)
		require.Equal(t, 404, r.Delete("/templates/absentprovisioner/name", JSON, "").StatusCode)

		// Duplicate template.
		response := r.Post("/templates/testcloud/prodtemplate/create", JSON, requireMarshalSuccess(t, template))
		require.Equal(t, 200, response.StatusCode)
		response = r.Post("/templates/testcloud/prodtemplate/create", JSON, requireMarshalSuccess(t, template))
		require.Equal(t, 409, response.StatusCode)
	})
}
