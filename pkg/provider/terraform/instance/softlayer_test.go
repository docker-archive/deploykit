package instance

import (
	"fmt"
	"testing"

	"github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/filter"
	"github.com/stretchr/testify/require"
)

func TestMergeLabelsIntoTagSliceEmpty(t *testing.T) {
	result := mergeLabelsIntoTagSlice([]interface{}{}, map[string]string{})
	require.Equal(t, []string{}, result)
}

func TestMergeLabelsIntoTagSliceTagsOnly(t *testing.T) {
	result := mergeLabelsIntoTagSlice(
		[]interface{}{
			"tag1:val1",
			"tag2:val2",
		},
		map[string]string{},
	)
	require.Len(t, result, 2)
	require.Contains(t, result, "tag1:val1")
	require.Contains(t, result, "tag2:val2")
}

func TestMergeLabelsIntoTagSliceLabelsOnly(t *testing.T) {
	result := mergeLabelsIntoTagSlice(
		[]interface{}{},
		map[string]string{
			"label1": "val1",
			"label2": "val2",
		},
	)
	require.Len(t, result, 2)
	require.Contains(t, result, "label1:val1")
	require.Contains(t, result, "label2:val2")
}

func TestMergeLabelsIntoTagSlice(t *testing.T) {
	result := mergeLabelsIntoTagSlice(
		[]interface{}{
			"tag1:val1",
			"tag2:val2",
		},
		map[string]string{
			"label1": "val1",
			"label2": "val2",
		},
	)
	require.Len(t, result, 4)
	require.Contains(t, result, "tag1:val1")
	require.Contains(t, result, "tag2:val2")
	require.Contains(t, result, "label1:val1")
	require.Contains(t, result, "label2:val2")
}

func TestFilterVMsByTagsEmpty(t *testing.T) {
	vms := []datatypes.Virtual_Guest{}
	filterVMsByTags(&vms, []string{})
	require.Equal(t, []datatypes.Virtual_Guest{}, vms)
}

func TestGetUniqueVMByTagsEmpty(t *testing.T) {
	id, err := getUniqueVMByTags([]datatypes.Virtual_Guest{}, []string{})
	require.NoError(t, err)
	require.Nil(t, id)
}

func TestGetUniqueVMByTagsOneMatch(t *testing.T) {
	vmID := 123
	vmTagName := "some-tag"
	vmTag := datatypes.Tag{Name: &vmTagName}
	vms := []datatypes.Virtual_Guest{{Id: &vmID, TagReferences: []datatypes.Tag_Reference{{Tag: &vmTag}}}}
	id, err := getUniqueVMByTags(vms, []string{vmTagName})
	require.NoError(t, err)
	require.Equal(t, vmID, *id)
}

func TestGetUniqueVMByTagsOneMatchNilID(t *testing.T) {
	vmTagName := "some-tag"
	vmHostname := "some-hostname"
	vmTag := datatypes.Tag{Name: &vmTagName}
	vms := []datatypes.Virtual_Guest{
		{
			Hostname:      &vmHostname,
			TagReferences: []datatypes.Tag_Reference{{Tag: &vmTag}},
		},
	}
	id, err := getUniqueVMByTags(vms, []string{vmTagName})
	require.Equal(t, "VM 'some-hostname' missing ID", err.Error())
	require.Nil(t, id)
}

func TestGetUniqueVMByTagsTwoMatches(t *testing.T) {
	vmID1 := 123
	vmID2 := 234
	vmTagName := "some-tag"
	vmTag := datatypes.Tag{Name: &vmTagName}
	vms := []datatypes.Virtual_Guest{
		{Id: &vmID1, TagReferences: []datatypes.Tag_Reference{{Tag: &vmTag}}},
		{Id: &vmID2, TagReferences: []datatypes.Tag_Reference{{Tag: &vmTag}}},
	}
	id, err := getUniqueVMByTags(vms, []string{vmTagName})
	require.Equal(t,
		fmt.Sprintf("Only a single VM should match tags, but VMs %v match tags: %v", []int{vmID1, vmID2}, []string{vmTagName}),
		err.Error())
	require.Nil(t, id)
}

// getVMs is a utility function to get datatypes.Virtual_Guest with tags
func getVMs() []datatypes.Virtual_Guest {
	vmID0 := 0
	vmID1 := 1
	vmID2 := 2
	vmID3 := 3
	tag1Name := "tag1"
	tag1 := datatypes.Tag{Name: &tag1Name}
	tag2Name := "tag2"
	tag2 := datatypes.Tag{Name: &tag2Name}
	tag3Name := "tag3"
	tag3 := datatypes.Tag{Name: &tag3Name}
	vms := []datatypes.Virtual_Guest{
		{
			TagReferences: []datatypes.Tag_Reference{},
			Id:            &vmID0,
		},
		{
			TagReferences: []datatypes.Tag_Reference{{Tag: &tag1}},
			Id:            &vmID1,
		},
		{
			TagReferences: []datatypes.Tag_Reference{{Tag: &tag1}, {Tag: &tag2}},
			Id:            &vmID2,
		},
		{
			TagReferences: []datatypes.Tag_Reference{{Tag: &tag1}, {Tag: &tag2}, {Tag: &tag3}},
			Id:            &vmID3,
		},
	}
	return vms
}

func TestFilterVMsByTags(t *testing.T) {
	// No tags given, everything matches
	vms := getVMs()
	filterVMsByTags(&vms, []string{})
	require.Len(t, vms, 4)
	require.Equal(t, 0, *vms[0].Id)
	require.Equal(t, 1, *vms[1].Id)
	require.Equal(t, 2, *vms[2].Id)
	require.Equal(t, 3, *vms[3].Id)
	// Empty tag, nothing matches
	vms = getVMs()
	filterVMsByTags(&vms, []string{""})
	require.Len(t, vms, 0)
	// 1 tag matches
	vms = getVMs()
	filterVMsByTags(&vms, []string{"tag1"})
	require.Len(t, vms, 3)
	require.Equal(t, 1, *vms[0].Id)
	require.Equal(t, 2, *vms[1].Id)
	require.Equal(t, 3, *vms[2].Id)
	// 2 tags match
	vms = getVMs()
	filterVMsByTags(&vms, []string{"tag1", "tag2"})
	require.Len(t, vms, 2)
	require.Equal(t, 2, *vms[0].Id)
	require.Equal(t, 3, *vms[1].Id)
	// 3 tags match
	vms = getVMs()
	filterVMsByTags(&vms, []string{"tag1", "tag2", "tag3"})
	require.Len(t, vms, 1)
	require.Equal(t, 3, *vms[0].Id)
	// A tag that doesn't match
	vms = getVMs()
	filterVMsByTags(&vms, []string{"tag1", "foo"})
	require.Len(t, vms, 0)
}

func TestGetIBMCloudVMByTagAPIError(t *testing.T) {
	fake := client.FakeSoftlayer{
		GetVirtualGuestsStub: func(mask, filters *string) (resp []datatypes.Virtual_Guest, err error) {
			return []datatypes.Virtual_Guest{}, fmt.Errorf("Custom error")
		},
	}
	b, err := GetIBMCloudVMByTag(&fake, nil)
	require.Error(t, err)
	require.Equal(t, "Custom error", err.Error())
	require.Empty(t, b)
}

func TestGetIBMCloudVMByTagNoVMs(t *testing.T) {
	vmID1 := 1
	vmID2 := 2
	tag1Val := "tag1:val1"
	tag1 := datatypes.Tag{Name: &tag1Val}
	tagClusterIDVal := flavor.ClusterIDTag + ":my-cluster-id"
	tag2 := datatypes.Tag{Name: &tagClusterIDVal}
	tag3Val := "tag1:no-match"
	tag3 := datatypes.Tag{Name: &tag3Val}
	fake := client.FakeSoftlayer{
		GetVirtualGuestsStub: func(mask, filters *string) (resp []datatypes.Virtual_Guest, err error) {
			return []datatypes.Virtual_Guest{
				{
					TagReferences: []datatypes.Tag_Reference{{Tag: &tag1}, {Tag: &tag2}},
					Id:            &vmID1,
				},
				{
					TagReferences: []datatypes.Tag_Reference{{Tag: &tag2}, {Tag: &tag3}},
					Id:            &vmID2,
				},
			}, nil
		},
	}
	tags := []string{"tag1:val1", flavor.ClusterIDTag + ":my-cluster-id"}
	id, err := GetIBMCloudVMByTag(&fake, tags)
	require.NoError(t, err)
	require.Equal(t, vmID1, *id)

	// Verify args
	expectedMask := "id,hostname,tagReferences[id,tag[name]]"
	expectedFilter := filter.New(filter.Path("virtualGuests.tagReferences.tag.name").Eq(flavor.ClusterIDTag + ":my-cluster-id")).Build()
	require.Equal(t,
		[]struct {
			Mask    *string
			Filters *string
		}{{&expectedMask, &expectedFilter}},
		fake.GetVirtualGuestsArgs)
}
