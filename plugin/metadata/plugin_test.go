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
	return &plugin{buildTopics(api)}
}

func TestList(t *testing.T) {
	api, _ := NewMockGCloud(t)

	plugin := NewPlugin(api)
	children, err := plugin.List(metadata.Path([]string{"path"}))

	require.EqualValues(t, []string{"1", "2"}, children)
	require.NoError(t, err)
}

func TestGetValues(t *testing.T) {
	api, _ := NewMockGCloud(t)

	plugin := NewPlugin(api)

	value, err := plugin.Get(metadata.Path([]string{"path", "1"}))
	require.EqualValues(t, `"value1"`, value.String())
	require.NoError(t, err)

	value, err = plugin.Get(metadata.Path([]string{"path", "2"}))
	require.EqualValues(t, `"value2"`, value.String())
	require.NoError(t, err)
}
