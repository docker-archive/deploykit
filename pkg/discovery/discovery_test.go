package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNotUnixSocketOrListener(t *testing.T) {
	err := ErrNotUnixSocketOrListener("no socket!")
	require.Error(t, err)
	require.True(t, IsErrNotUnixSocketOrListener(err))
}
