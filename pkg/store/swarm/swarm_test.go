package swarm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {

	input := map[string]interface{}{
		"Group": map[string]interface{}{
			"managers": map[string]interface{}{
				"Instance":   "foo",
				"Flavor":     "bar",
				"Allocation": []interface{}{"a", "b", "c"},
			},
			"workers": map[string]interface{}{
				"Instance": "bar",
				"Flavor":   "baz",
			},
		},
	}

	encoded, err := encode(input)
	require.NoError(t, err)
	t.Log("encoded=", encoded)

	output := map[string]interface{}{}
	err = decode(encoded, &output)
	require.NoError(t, err)

	require.Equal(t, input, output)
}
