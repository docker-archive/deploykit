package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWatches(t *testing.T) {

	w := &Watch{}

	test1 := make(chan struct{})
	test2 := make(chan struct{})
	w.Add("test", test1)
	w.Add("test", test2)

	require.Equal(t, 2, len(w.watchers["test"]))

	w.Notify("none")

	w.Notify("test")
	<-test1
	<-test2

	require.Nil(t, w.watchers["test"])
}

func TestWatchers(t *testing.T) {

	w := &Watch{}

	test1 := make(chan struct{})
	test2 := make(chan struct{})
	w.Add("test1", test1)
	w.Add("test2", test2)

	watchers := append(Watchers{}, test1, test2)

	w.Notify("test")

	w.Notify("test1")

	ctx := context.Background()
	all := watchers.FanIn(ctx)

	select {
	case <-all:
		require.Fail(t, "shouldn't fire this early")
	default:
	}

	w.Notify("test2")
	<-all

}

func TestWatchersWithCancel(t *testing.T) {

	w := &Watch{}

	test1 := make(chan struct{})
	test2 := make(chan struct{})
	w.Add("test1", test1)
	w.Add("test2", test2)

	watchers := append(Watchers{}, test1, test2)

	w.Notify("test")

	w.Notify("test1")

	ctx, cancel := context.WithCancel(context.Background())
	all := watchers.FanIn(ctx)

	select {
	case <-all:
		require.Fail(t, "shouldn't fire this early")
	default:
	}

	cancel()
	<-all // shouldn't block
}
