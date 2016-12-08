package gcloud_test

import (
	"testing"

	"github.com/docker/infrakit.gcp/plugin/instance/gcloud"
	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	tags := map[string]string{
		"infrakit.key":        "value1",
		"infrakit.custom-key": "value2",
		"infrakit.group.key":  "value3",
	}

	metaData := gcloud.TagsToMetaData(tags)
	tagsFromMetata := gcloud.MetaDataToTags(metaData)

	require.Equal(t, tags, tagsFromMetata)
}

func TestConvertEmpty(t *testing.T) {
	tags := map[string]string{}

	metaData := gcloud.TagsToMetaData(tags)
	tagsFromMetata := gcloud.MetaDataToTags(metaData)

	require.Empty(t, tagsFromMetata)
}

func TestConvertNil(t *testing.T) {
	metaData := gcloud.TagsToMetaData(nil)
	tagsFromMetata := gcloud.MetaDataToTags(metaData)

	require.Empty(t, tagsFromMetata)
}
