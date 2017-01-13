package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLookupAndType(t *testing.T) {
	lookup, instanceType := GetLookupAndType("instance-file")
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "", instanceType)

	lookup, instanceType = GetLookupAndType("instance-file/json")
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "json", instanceType)

	lookup, instanceType = GetLookupAndType("instance-file/text/html")
	require.Equal(t, "instance-file", lookup)
	require.Equal(t, "text/html", instanceType)
}
