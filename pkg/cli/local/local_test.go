package local

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkip(t *testing.T) {
	require.True(t, skip(".foo"))
	require.True(t, skip(".foo~"))
	require.True(t, skip("file~"))
	require.False(t, skip("file"))
	require.True(t, skip("README.md"))
}
