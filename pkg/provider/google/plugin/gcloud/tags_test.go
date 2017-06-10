package gcloud

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	tags := map[string]string{
		"infrakit.key":        "value1",
		"infrakit.custom-key": "value2",
		"infrakit.group.key":  "value3",
	}

	metaData := TagsToMetaData(tags)
	tagsFromMetata := MetaDataToTags(metaData)

	require.Equal(t, tags, tagsFromMetata)
}

func TestConvertEmpty(t *testing.T) {
	tags := map[string]string{}

	metaData := TagsToMetaData(tags)
	tagsFromMetata := MetaDataToTags(metaData)

	require.Empty(t, tagsFromMetata)
}

func TestConvertNil(t *testing.T) {
	metaData := TagsToMetaData(nil)
	tagsFromMetata := MetaDataToTags(metaData)

	require.Empty(t, tagsFromMetata)
}
