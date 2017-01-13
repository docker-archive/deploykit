package plugin

import (
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
