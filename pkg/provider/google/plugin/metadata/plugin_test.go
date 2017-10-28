package metadata

import (
	"testing"

	mock_gcloud "github.com/docker/infrakit/pkg/provider/google/mock/gcloud"
	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
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

func TestKeys(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)

	plugin := NewPlugin(api, apiMetadata)
	children, err := plugin.Keys(types.Path([]string{""}))

	require.EqualValues(t, []string{"instance", "project", "zone"}, children)
	require.NoError(t, err)
}

func TestListInstance(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)

	plugin := NewPlugin(api, apiMetadata)
	children, err := plugin.Keys(types.Path([]string{"instance"}))

	require.EqualValues(t, []string{"ID", "externalIP", "hostname", "internalIP", "name", "network", "numericalProjectID", "projectID", "zone"}, children)
	require.NoError(t, err)
}

func TestGetProject(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	api.EXPECT().GetProject().Return("PROJECT")

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"project"}))

	require.EqualValues(t, `"PROJECT"`, value.String())
	require.NoError(t, err)
}

func TestGetZone(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	api.EXPECT().GetZone().Return("ZONE")

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"zone"}))

	require.EqualValues(t, `"ZONE"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceProjectID(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().ProjectID().Return("PROJECT_ID", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "projectID"}))

	require.EqualValues(t, `"PROJECT_ID"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceNumericProjectID(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().NumericProjectID().Return("421337", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "numericalProjectID"}))

	require.EqualValues(t, `"421337"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceInternalIP(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().InternalIP().Return("10.0.0.1", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "internalIP"}))

	require.EqualValues(t, `"10.0.0.1"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceExternalIP(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().ExternalIP().Return("134.45.45.1", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "externalIP"}))

	require.EqualValues(t, `"134.45.45.1"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceHostname(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().Hostname().Return("HOSTNAME", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "hostname"}))

	require.EqualValues(t, `"HOSTNAME"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceID(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().InstanceID().Return("ID", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "ID"}))

	require.EqualValues(t, `"ID"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceName(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().InstanceName().Return("NAME", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "name"}))

	require.EqualValues(t, `"NAME"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceZone(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().Zone().Return("ZONE", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "zone"}))

	require.EqualValues(t, `"ZONE"`, value.String())
	require.NoError(t, err)
}

func TestGetInstanceNetwork(t *testing.T) {
	api, apiMetadata, _ := NewMockAPI(t)
	apiMetadata.EXPECT().Get("instance/network-interfaces/0/network").Return("path/to/network", nil)

	plugin := NewPlugin(api, apiMetadata)
	value, err := plugin.Get(types.Path([]string{"instance", "network"}))

	require.EqualValues(t, `"network"`, value.String())
	require.NoError(t, err)
}
