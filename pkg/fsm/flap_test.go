package fsm

import (
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/require"
)

func index(a int, b ...int) []Index {
	out := []Index{Index(a)}
	for _, bb := range b {
		out = append(out, Index(bb))
	}
	return out
}

func TestHistory(t *testing.T) {
	require.True(t, equals(index(1, 2, 3), index(1, 2, 3)))
	require.False(t, equals(index(2, 3), index(1, 2, 3)))
}

func TestFlap(t *testing.T) {

	const (
		a Index = iota
		b
		c
		x
		y
	)

	counter := newFlaps()
	require.Equal(t, 0, counter.count(a, b))

	counter.record(a, b)
	glog.Infoln(counter.history)

	counter.record(b, a)
	glog.Infoln(counter.history)

	counter.record(a, c)
	glog.Infoln(counter.history)

	counter.record(a, b)
	glog.Infoln(counter.history)

	counter.record(b, a)
	glog.Infoln(counter.history)

	counter.record(a, b)
	glog.Infoln(counter.history)

	counter.record(b, a)
	glog.Infoln(counter.history)

	require.Equal(t, 2, counter.count(a, b))
	require.Equal(t, 2, counter.count(b, a))

	counter.record(a, b)
	glog.Infoln(counter.history)

	counter.record(b, a)
	glog.Infoln(counter.history)

	require.Equal(t, 3, counter.count(a, b))
}
