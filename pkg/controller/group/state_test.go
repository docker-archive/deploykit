package group

import (
	"sort"
	"testing"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

func TestSortByID(t *testing.T) {

	{
		list := []instance.Description{
			{ID: "d", LogicalID: logicalID("4")},
			{ID: "c", LogicalID: logicalID("3")},
			{ID: "b", LogicalID: logicalID("2")},
			{ID: "a", LogicalID: logicalID("1")},
		}

		sort.Sort(sortByID{list: list})

		require.Equal(t,
			[]instance.Description{
				{ID: "a", LogicalID: logicalID("1")},
				{ID: "b", LogicalID: logicalID("2")},
				{ID: "c", LogicalID: logicalID("3")},
				{ID: "d", LogicalID: logicalID("4")},
			},
			list)

	}
	{
		list := []instance.Description{
			{ID: "d", LogicalID: logicalID("4")},
			{ID: "c", LogicalID: logicalID("3")},
			{ID: "b", LogicalID: logicalID("2")},
			{ID: "a", LogicalID: logicalID("1")},
		}

		sort.Sort(sortByID{list: list,
			settings: &groupSettings{
				self:    logicalID("1"),
				options: group_types.Options{}}})

		require.Equal(t,
			[]instance.Description{
				{ID: "b", LogicalID: logicalID("2")},
				{ID: "c", LogicalID: logicalID("3")},
				{ID: "d", LogicalID: logicalID("4")},
				{ID: "a", LogicalID: logicalID("1")},
			},
			list)

	}
	{
		list := []instance.Description{
			{ID: "d", LogicalID: logicalID("4")},
			{ID: "c", LogicalID: logicalID("3")},
			{ID: "b", LogicalID: logicalID("2")},
			{ID: "a", LogicalID: logicalID("1")},
		}

		sort.Sort(sortByID{list: list,
			settings: &groupSettings{
				self:    logicalID("3"),
				options: group_types.Options{}}})

		require.Equal(t,
			[]instance.Description{
				{ID: "a", LogicalID: logicalID("1")},
				{ID: "b", LogicalID: logicalID("2")},
				{ID: "d", LogicalID: logicalID("4")},
				{ID: "c", LogicalID: logicalID("3")},
			},
			list)

	}

	tagsWithLogicalID := func(v string) map[string]string {
		return map[string]string{
			instance.LogicalIDTag: v,
		}
	}
	{
		list := []instance.Description{
			{ID: "d", Tags: tagsWithLogicalID("4")},
			{ID: "c", Tags: tagsWithLogicalID("3")},
			{ID: "b", Tags: tagsWithLogicalID("2")},
			{ID: "a", Tags: tagsWithLogicalID("1")},
		}

		sort.Sort(sortByID{list: list,
			settings: &groupSettings{
				self:    logicalID("1"),
				options: group_types.Options{}}})

		require.Equal(t,
			[]instance.Description{
				{ID: "b", Tags: tagsWithLogicalID("2")},
				{ID: "c", Tags: tagsWithLogicalID("3")},
				{ID: "d", Tags: tagsWithLogicalID("4")},
				{ID: "a", Tags: tagsWithLogicalID("1")},
			},
			list)

	}
}
