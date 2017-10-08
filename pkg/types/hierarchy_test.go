package types

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHierarchy(t *testing.T) {

	model := map[string]interface{}{}

	h := HierarchicalFromMap(model)

	require.True(t, Put(PathFromString("a/b/c/d"), "abcd", model))
	require.True(t, Put(PathFromString("a/x/y/z"), "axyz", model))
	require.True(t, Put(PathFromString("k/w/u"), "kwu", model))
	require.True(t, Put(PathFromString("k/p"), "kp", model))
	require.True(t, Put(PathFromString("k/r"), 25, model))

	l, err := h.List(PathFromString("."))
	sort.Strings(l)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "k"}, l)

	any, err := h.Get(PathFromString("a/b/c/d"))
	require.NoError(t, err)
	require.Equal(t, "\"abcd\"", any.String())

	any, err = h.Get(PathFromString("a/x/y/z"))
	require.NoError(t, err)
	require.Equal(t, "\"axyz\"", any.String())

	any, err = h.Get(PathFromString("k/r"))
	require.NoError(t, err)
	require.Equal(t, "25", any.String())

	all, err := ListAll(h, PathFromString("."))
	require.NoError(t, err)

	SortPaths(all)
	require.Equal(t, Paths{
		{"a"},
		{"a", "b"},
		{"a", "b", "c"},
		{"a", "b", "c", "d"},
		{"a", "x"},
		{"a", "x", "y"},
		{"a", "x", "y", "z"},
		{"k"},
		{"k", "p"},
		{"k", "r"},
		{"k", "w"},
		{"k", "w", "u"},
	}, all)
}
