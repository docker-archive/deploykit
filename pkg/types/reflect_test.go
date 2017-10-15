package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func pretty(v interface{}) string {
	buff, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(buff)
}

func TestTokenizer(t *testing.T) {
	require.Equal(t, []string{}, tokenize(""))
	require.Equal(t, []string{"."}, tokenize("."))
	require.Equal(t, []string{"a"}, tokenize("a"))
	require.Equal(t, []string{"", "foo"}, tokenize("/foo"))
	require.Equal(t, []string{"", "foo", "bar", "baz"}, tokenize("/foo/bar/baz"))
	require.Equal(t, []string{"foo", "bar", "baz"}, tokenize("foo/bar/baz"))
	require.Equal(t, []string{"foo"}, tokenize("foo"))

	// with quoting to support azure rm type names: e.g. Microsoft.Network/virtualNetworks
	require.Equal(t, []string{"", "foo"}, tokenize("/'fo'o"))
	require.Equal(t, []string{"", "foo/bar", "baz"}, tokenize("/'foo/bar'/baz"))
	require.Equal(t, []string{"foo", "bar/baz"}, tokenize("foo/'bar/baz'"))
	require.Equal(t, []string{"foo"}, tokenize("'foo'"))
}

func TestGetSetDot(t *testing.T) {
	m := map[string]interface{}{
		"key1": 1,
		"key2": "key2",
	}
	require.Equal(t, m, Get(Dot, m))

	m2 := m
	m2["key2"] = "key2a"
	m2["key3"] = "key3"

	// Note we use &m to allow mutation of the entire struct when setting with path == Dot
	require.True(t, Put(Dot, m2, &m))
	require.Equal(t, m2, Get(Dot, m))
	require.Equal(t, m2, m)

	_, has := m["."]
	require.False(t, has)

	anym := AnyValueMust(m)
	require.True(t, Put(Dot, anym, &m))
	require.Equal(t, m, Get(Dot, m))
}

func TestMap(t *testing.T) {
	m := map[string]interface{}{}
	require.True(t, put(PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m))
	require.True(t, put(PathFromString("region/us-west-1/vpc/vpc1/network/network2/id"), "id-network2", m))
	require.True(t, put(PathFromString("region/us-west-1/vpc/vpc1/network/network3/id"), "id-network3", m))
	require.True(t, put(PathFromString("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m))
	require.True(t, put(PathFromString("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m))
	require.True(t, put(PathFromString("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m))
	require.True(t, put(PathFromString("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m))
	require.True(t, put(PathFromString("region/us-west-2/metrics/instances/count"), 100, m))
	require.True(t, put(PathFromString("region/us-west-2/instances"), AnyValueMust([]string{"a", "b"}), m))

	require.Equal(t, "id-network1", Get(PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), m))
	require.Equal(t, "id-network1", Get(PathFromString("region/us-west-1/vpc/vpc1/network/network1/id/"), m))
	require.Equal(t, map[string]interface{}{"id": "id-network1"},
		get(PathFromString("region/us-west-1/vpc/vpc1/network/network1"), m))
	require.Equal(t, map[string]interface{}{
		"network1": map[string]interface{}{
			"id": "id-network1",
		},
		"network2": map[string]interface{}{
			"id": "id-network2",
		},
		"network3": map[string]interface{}{
			"id": "id-network3",
		},
	}, get(PathFromString("region/us-west-1/vpc/vpc1/network/"), m))

	require.Equal(t, []string{"region"}, List(PathFromString("."), m))
	require.Equal(t, []string{}, List(PathFromString("region/us-west-1/vpc/vpc1/network/network1/id"), m))
	require.Equal(t, []string{"id"}, List(PathFromString("region/us-west-1/vpc/vpc1/network/network1"), m))
	require.Equal(t, []string{"us-west-1", "us-west-2"}, List(PathFromString("region/"), m))
	require.Equal(t, []string{"us-west-1", "us-west-2"}, List(PathFromString("region"), m))
	require.Equal(t, []string{"network1", "network2", "network3"}, List(PathFromString("region/us-west-1/vpc/vpc1/network/"), m))
	require.Equal(t, []string{}, List(PathFromString("region/us-west-2/metrics/instances/count"), m))
	require.Equal(t, []string{"[0]", "[1]"}, List(PathFromString("region/us-west-2/instances"), m))
	require.Equal(t, []string{}, List(PathFromString("region/us-west-2/instances/[0]"), m))
	require.Equal(t, "a", Get(PathFromString("region/us-west-2/instances/[0]"), m))
	require.Equal(t, "b", Get(PathFromString("region/us-west-2/instances/[1]"), m))
	require.Equal(t, "a", Get(PathFromString("region/us-west-2/instances[0]"), m))
	require.Equal(t, "b", Get(PathFromString("region/us-west-2/instances[1]"), m))

	// noexistent value list returns nil
	require.Equal(t, []string(nil), List(PathFromString("region-not-found"), m))
	require.Equal(t, []string(nil), List(PathFromString("region/us-west-1/bogus"), m))
	require.Equal(t, []string(nil), List(PathFromString("region/us-west-2/nonexist"), m))

	// existing non-list value list returns empty list
	require.Equal(t, 100, get(PathFromString("region/us-west-2/metrics/instances/count"), m))
	require.Equal(t, []string{}, List(PathFromString("region/us-west-2/metrics/instances/count"), m))

}

func TestGetFromStruct(t *testing.T) {

	type metric struct {
		Name  string
		Value int
	}

	type region struct {
		Metrics map[string]metric
		Values  map[string]interface{}
		Funcs   []interface{}
	}

	func1Called := make(chan struct{})
	func2Called := make(chan struct{})
	func3Called := make(chan struct{})

	func1 := func() interface{} {
		defer close(func1Called)
		return "func1"
	}

	func2 := func() interface{} {
		defer close(func2Called)
		return "func2"
	}

	func3 := func() interface{} {
		defer close(func3Called)
		return "func3"
	}

	func4 := func() interface{} {
		return []string{"func4"}
	}

	func5 := func() interface{} {
		return map[string]interface{}{
			"func5": 100,
			"func6": false,
		}
	}

	m := map[string]region{
		"us-west-1": {
			Metrics: map[string]metric{
				"instances": {Name: "instances", Value: 2000},
				"subnets":   {Name: "subnets", Value: 20},
			},
			Values: map[string]interface{}{
				"func1": func1,
				"func2": func2,
			},
			Funcs: []interface{}{func3, func4, func5},
		},
		"us-west-2": {
			Metrics: map[string]metric{
				"instances": {Name: "instances", Value: 4000},
				"subnets":   {Name: "subnets", Value: 40},
			},
		},
	}

	require.Equal(t, nil, Get(PathFromString("us-west-1/Metrics/instances/Count"), m))
	require.Equal(t, 2000, Get(PathFromString("us-west-1/Metrics/instances/Value"), m))
	require.Equal(t, "func1", Get(PathFromString("us-west-1/Values/func1"), m))
	require.Equal(t, "func2", Get(PathFromString("us-west-1/Values/func2"), m))
	require.Equal(t, "func3", Get(PathFromString("us-west-1/Funcs[0]"), m))

	<-func1Called
	<-func2Called
	<-func3Called

	require.Equal(t, []string{"[0]"}, List(PathFromString("us-west-1/Funcs/[1]"), m))
	require.Equal(t, "func4", Get(PathFromString("us-west-1/Funcs/[1]/[0]"), m))
	require.Equal(t, 100, Get(PathFromString("us-west-1/Funcs/[2]/func5"), m))
	require.Equal(t, []string{"[0]"}, List(PathFromString("us-west-1/Funcs/[1]"), m))
	require.Equal(t, []string{"func5", "func6"}, List(PathFromString("us-west-1/Funcs/[2]"), m))
	require.Equal(t, "func4", Get(PathFromString("us-west-1/Funcs/[1]/[0]"), m))

}
