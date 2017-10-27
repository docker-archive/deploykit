package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrVersionMismatch(t *testing.T) {
	var e error

	e = errVersionMismatch("test")
	require.True(t, IsErrVersionMismatch(e))

	e = fmt.Errorf("untyped")
	require.False(t, IsErrVersionMismatch(e))
}
