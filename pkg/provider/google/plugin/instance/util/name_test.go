package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomNameSuffixLen(t *testing.T) {
	require.Len(t, RandomSuffix(1), 1)
	require.Len(t, RandomSuffix(4), 4)
	require.Len(t, RandomSuffix(6), 6)
	require.Len(t, RandomSuffix(10), 10)
}

func TestRandomNameSuffix(t *testing.T) {
	require.NotEqual(t, RandomSuffix(8), RandomSuffix(8))
}
