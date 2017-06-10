package gcloud

import (
	"sort"
	"strings"

	"google.golang.org/api/compute/v1"
)

// TagsToMetaData converts a tag map into VM Metadata items.
func TagsToMetaData(tags map[string]string) []*compute.MetadataItems {
	items := []*compute.MetadataItems{}

	for k, v := range tags {
		valueCopy := v
		items = append(items, &compute.MetadataItems{
			Key:   escapeKey(k),
			Value: &valueCopy,
		})
	}

	sort.Sort(ByKey(items))

	return items
}

// MetaDataToTags converts VM Metadata items into a tag map.
func MetaDataToTags(metaData []*compute.MetadataItems) map[string]string {
	tags := map[string]string{}

	for _, item := range metaData {
		key := unEscapeKey(item.Key)
		tags[key] = *item.Value
	}

	return tags
}

// HasDifferentTag compares two sets of tags.
func HasDifferentTag(expected, actual map[string]string) bool {
	for k, v := range expected {
		if actual[k] != v {
			return true
		}
	}

	return false
}

func escapeKey(key string) string {
	return strings.Replace(key, ".", "--", -1)
}

func unEscapeKey(escapedKey string) string {
	return strings.Replace(escapedKey, "--", ".", -1)
}

// ByKey implements sort.Interface for []*compute.MetadataItems based on
// the Key field.
type ByKey []*compute.MetadataItems

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }
