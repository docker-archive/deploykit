package metadata

import (
	"testing"

	mock_gcloud "github.com/docker/infrakit.gcp/mock/gcloud"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func NewMockGCloud(t *testing.T) (*mock_gcloud.MockAPI, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mock_gcloud.NewMockAPI(ctrl), ctrl
}

func NewPlugin(api gcloud.API) metadata.Plugin {
	return &plugin{api: api}
}

func TestList(t *testing.T) {
	api, _ := NewMockGCloud(t)

	plugin := NewPlugin(api)
	children, err := plugin.List(metadata.Path([]string{""}))

	require.EqualValues(t, []string{"project", "zone"}, children)
	require.NoError(t, err)
}

func TestGetProject(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().GetProject().Return("PROJECT")

	plugin := NewPlugin(api)
	value, err := plugin.Get(metadata.Path([]string{"project"}))

	require.EqualValues(t, `"PROJECT"`, value.String())
	require.NoError(t, err)
}

func TestGetZone(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().GetZone().Return("ZONE")

	plugin := NewPlugin(api)
	value, err := plugin.Get(metadata.Path([]string{"zone"}))

	require.EqualValues(t, `"ZONE"`, value.String())
	require.NoError(t, err)
}
