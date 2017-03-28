package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNotUnixSocket(t *testing.T) {
	err := ErrNotUnixSocket("no socket!")
	require.Error(t, err)
	require.True(t, IsErrNotUnixSocket(err))
}
