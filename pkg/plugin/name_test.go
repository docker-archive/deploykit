package plugin

import (
	"encoding/json"
	"testing"

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
	Plugin     Name `json:"plugin"`
	Properties *Any `json:"properties,omitempty"`
}

type testSpec struct {
	Plugin     Name      `json:"plugin"`
	Properties *Any      `json:"properties,omitempty"`
	Nested     testSpec2 `json:"nested"`
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
}
