package plugin

import (
	"encoding/json"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestGetLookupAndType(t *testing.T) {

	ref := Name("instance-file")
	lookup, instanceType := ref.GetLookupAndType()
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "", instanceType)

	ref = Name("instance-file/json")
	lookup, instanceType = ref.GetLookupAndType()
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "json", instanceType)

	ref = Name("instance-file/text/html")
	lookup, instanceType = ref.GetLookupAndType()
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "text/html", instanceType)
}

type testSpec2 struct {
	Plugin Name `json:"plugin"`
}

type testSpec struct {
	Plugin Name      `json:"plugin"`
	Nested testSpec2 `json:"nested"`
}

func TestMarshalUnmarshal(t *testing.T) {

	spec := testSpec{}

	err := json.Unmarshal([]byte(`{  "plugin" : "instance-file/json" }`), &spec)
	require.NoError(t, err)
	require.Equal(t, "instance-file/json", spec.Plugin.String())

	err = json.Unmarshal([]byte(`{  "plugin" : "instance-file" }`), &spec)
	require.NoError(t, err)
	require.Equal(t, "instance-file", spec.Plugin.String())

	err = json.Unmarshal([]byte(`{  "plugin" : 100 }`), &spec)
	require.Error(t, err)
	require.Equal(t, "bad-format-for-name:100", err.Error())

	err = json.Unmarshal([]byte(`{  "plugin" : "instance-file", "nested" : { "plugin": "instance-file/nested"} }`), &spec)
	require.NoError(t, err)
	require.Equal(t, "instance-file", spec.Plugin.String())
	require.Equal(t, "instance-file/nested", spec.Nested.Plugin.String())

	// marshal

	spec.Plugin = Name("instance-file/text")
	buff, err := json.Marshal(spec)
	require.NoError(t, err)
	require.Equal(t, `{"plugin":"instance-file/text","nested":{"plugin":"instance-file/nested"}}`, string(buff))

	// marshal using Any

	complex := map[string]interface{}{
		"Plugin": Name("top"),
		"Properties": types.AnyValueMust(testSpec{
			Plugin: Name("test-plugin1"),
			Nested: testSpec2{
				Plugin: Name("nested1"),
			},
		}),
	}

	any := types.AnyValueMust(complex)

	parsed := map[string]interface{}{}
	err = json.Unmarshal(any.Bytes(), &parsed)
	require.NoError(t, err)

	require.Equal(t, "top", parsed["Plugin"])
	require.Equal(t, "nested1", parsed["Properties"].(map[string]interface{})["nested"].(map[string]interface{})["plugin"])
}
