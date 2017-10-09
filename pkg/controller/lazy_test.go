package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLazyBlockAndCancel(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (Controller, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 100*time.Second)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Describe(nil)
		errs <- err
		close(errs)
	}()

	<-called

	CancelWait(g)

	require.Equal(t, "cancelled", (<-errs).Error())
}

func TestLazyNoBlock(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (Controller, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Describe(nil)
		errs <- err
		close(errs)
	}()

	<-called

	require.Equal(t, "boom", (<-errs).Error())
}

type fake chan int

func (f fake) Plan(op Operation, spec types.Spec) (object types.Object, plan Plan, err error) {
	f <- 1
	return
}

func (f fake) Commit(op Operation, spec types.Spec) (object types.Object, err error) {
	f <- 1
	return
}

func (f fake) Describe(search *types.Metadata) (objects []types.Object, err error) {
	f <- 1
	return
}

func (f fake) Free(search *types.Metadata) (objects []types.Object, err error) {
	f <- 1
	return
}

func TestLazyNoBlockConnect(t *testing.T) {

	called := make(chan struct{})
	called2 := make(chan int, 2)

	g := LazyConnect(func() (Controller, error) {
		close(called)
		return fake(called2), nil
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Describe(nil)
		errs <- err
		close(errs)

		g.Free(nil)
		close(called2)
	}()

	<-called

	require.NoError(t, <-errs)
	calls := 0
	for n := range called2 {
		calls += n
	}
	require.Equal(t, 2, calls)
}
