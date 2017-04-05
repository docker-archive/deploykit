package metadata

import (
	"testing"

	mock_gcloud "github.com/docker/infrakit.gcp/mock/gcloud"
	"github.com/docker/infrakit.gcp/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func NewMockAPI(t *testing.T) (*mock_gcloud.MockAPI, *mock_gcloud.MockAPIMetadata, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mock_gcloud.NewMockAPI(ctrl), mock_gcloud.NewMockAPIMetadata(ctrl), ctrl
}

func NewPlugin(api gcloud.API, apiMetadata gcloud.APIMetadata) metadata.Plugin {
	return &plugin{
		api:         api,
		apiMetadata: apiMetadata,
	}
}

func TestList(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)

	plugin := NewPlugin(api, apiMetadata)
	children, err := plugin.List(metadata.Path([]string{""}))

	require.EqualValues(t, []string{"instance", "project", "zone"}, children)
	require.NoError(t, err)
}

func TestListInstance(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)

	plugin := NewPlugin(api, apiMetadata)
	children, err := plugin.List(metadata.Path([]string{"instance"}))

	require.EqualValues(t, []string{"hostname"}, children)
	require.NoError(t, err)
}

func TestGetProject(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	api.EXPECT().GetProject().Return("PROJECT")

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(metadata.Path([]string{"project"}))

	require.EqualValues(t, `"PROJECT"`, value.String())
	require.NoError(t, err)
}

func TestGetZone(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	api.EXPECT().GetZone().Return("ZONE")

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(metadata.Path([]string{"zone"}))

	require.EqualValues(t, `"ZONE"`, value.String())
	require.NoError(t, err)
}

func TestGetHostname(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().GetHostname().Return("HOSTNAME", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(metadata.Path([]string{"instance", "hostname"}))

	require.EqualValues(t, `"HOSTNAME"`, value.String())
	require.NoError(t, err)
}
