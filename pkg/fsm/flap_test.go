package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlap(t *testing.T) {

	const (
		a Index = iota
		b
		c
		x
		y
	)

	counter := &flaps{index: map[[2]Index]int{}}
	require.Equal(t, 0, counter.count(a, b))

	counter.record(a, b)
	counter.record(b, a)
	require.Equal(t, 1, counter.count(a, b))

	counter.record(a, b)
	require.Equal(t, 1, counter.count(a, b))
	counter.record(b, a)
	require.Equal(t, 2, counter.count(a, b))
	counter.record(a, b)
	counter.record(b, a)
	require.Equal(t, 3, counter.count(a, b))

	counter.reset(a, b)
	require.Equal(t, 0, counter.count(b, a))
	require.Equal(t, 0, counter.count(a, b))

	counter.record(x, y)
	require.Equal(t, 0, counter.count(x, y))
	require.Equal(t, 0, counter.count(y, x))
	counter.record(y, x)
	require.Equal(t, 1, counter.count(x, y))
	require.Equal(t, 1, counter.count(y, x))
	counter.record(x, y)
	require.Equal(t, 1, counter.count(x, y))
	counter.record(y, x)
	require.Equal(t, 2, counter.count(x, y))

	counter.reset(x, y)
	for i := 0; i < 5; i++ {
		counter.record(x, y)
		// some time later
		counter.record(y, x)
	}
	require.Equal(t, 5, counter.count(y, x))
}
