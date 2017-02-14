package metadata

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

func TestMap(t *testing.T) {
	m := map[string]interface{}{}
	require.True(t, put(Path("region/us-west-1/vpc/vpc1/network/network1/id"), "id-network1", m))
	require.True(t, put(Path("region/us-west-1/vpc/vpc1/network/network2/id"), "id-network2", m))
	require.True(t, put(Path("region/us-west-1/vpc/vpc1/network/network3/id"), "id-network3", m))
	require.True(t, put(Path("region/us-west-1/vpc/vpc2/network/network10/id"), "id-network10", m))
	require.True(t, put(Path("region/us-west-1/vpc/vpc2/network/network11/id"), "id-network11", m))
	require.True(t, put(Path("region/us-west-2/vpc/vpc21/network/network210/id"), "id-network210", m))
	require.True(t, put(Path("region/us-west-2/vpc/vpc21/network/network211/id"), "id-network211", m))
	require.True(t, put(Path("region/us-west-2/metrics/instances/count"), 100, m))

	require.Equal(t, "id-network1", get(Path("region/us-west-1/vpc/vpc1/network/network1/id"), m))
	require.Equal(t, "id-network1", get(Path("region/us-west-1/vpc/vpc1/network/network1/id/"), m))
	require.Equal(t, map[string]interface{}{"id": "id-network1"},
		get(Path("region/us-west-1/vpc/vpc1/network/network1"), m))
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
	}, get(Path("region/us-west-1/vpc/vpc1/network/"), m))

	require.Equal(t, 100, get(Path("region/us-west-2/metrics/instances/count"), m))

	require.Equal(t, []string{}, List(Path("region/us-west-1/bogus"), m))
	require.Equal(t, []string{}, List(Path("region/us-west-1/vpc/vpc1/network/network1/id"), m))
	require.Equal(t, []string{"id"}, List(Path("region/us-west-1/vpc/vpc1/network/network1"), m))
	require.Equal(t, []string{"us-west-1", "us-west-2"}, List(Path("region/"), m))
	require.Equal(t, []string{"us-west-1", "us-west-2"}, List(Path("region"), m))
	require.Equal(t, []string{"network1", "network2", "network3"}, List(Path("region/us-west-1/vpc/vpc1/network/"), m))
	require.Equal(t, []string{}, List(Path("region/us-west-2/metrics/instances/count"), m))

}

func TestGetFromStruct(t *testing.T) {

	type metric struct {
		Name  string
		Value int
	}

	type region struct {
		Metrics map[string]metric
	}

	m := map[string]region{
		"us-west-1": {
			Metrics: map[string]metric{
				"instances": {Name: "instances", Value: 2000},
				"subnets":   {Name: "subnets", Value: 20},
			},
		},
		"us-west-2": {
			Metrics: map[string]metric{
				"instances": {Name: "instances", Value: 4000},
				"subnets":   {Name: "subnets", Value: 40},
			},
		},
	}

	require.Equal(t, nil, Get(Path("us-west-1/Metrics/instances/Count"), m))
	require.Equal(t, 2000, Get(Path("us-west-1/Metrics/instances/Value"), m))

}
