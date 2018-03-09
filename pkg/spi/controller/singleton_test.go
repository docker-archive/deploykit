package controller

import (
	"net/url"
	"testing"

	"github.com/docker/infrakit/pkg/spi/stack"
	"github.com/stretchr/testify/require"
)

// fakeLeader returns a fake leadership func
func fakeLeader(v bool) func() stack.Leadership {
	return func() stack.Leadership { return fakeLeaderT(v) }
}

type fakeLeaderT bool

func (f fakeLeaderT) IsLeader() (bool, error) {
	return bool(f), nil
}

func (f fakeLeaderT) LeaderLocation() (*url.URL, error) {
	return nil, nil
}

func TestSingleton(t *testing.T) {

	call1 := make(chan int, 1)
	singleton1 := Singleton(fake(call1), fakeLeader(true))

	_, err := singleton1.Describe(nil)
	require.NoError(t, err)
	require.Equal(t, 1, <-call1)

	call2 := make(chan int, 1)
	singleton2 := Singleton(fake(call2), fakeLeader(false))

	_, err = singleton2.Describe(nil)
	require.Error(t, err)
	require.Equal(t, "not a leader", err.Error())

	close(call1)
	close(call2)
}
