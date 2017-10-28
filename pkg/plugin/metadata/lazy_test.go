package metadata

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLazyBlockAndCancel(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (metadata.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 100*time.Second)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Get(types.PathFromString("a/b/c"))
		errs <- err
		close(errs)
	}()

	<-called

	CancelWait(g)

	require.Equal(t, "cancelled", (<-errs).Error())
}

func TestLazyNoBlock(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (metadata.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Get(types.PathFromString("a/b/c"))
		errs <- err
		close(errs)
	}()

	<-called

	require.Equal(t, "boom", (<-errs).Error())
}

type fake chan int

func (f fake) Keys(path types.Path) (child []string, err error) {
	f <- 1
	return
}

func (f fake) Get(path types.Path) (value *types.Any, err error) {
	f <- 1
	return
}

func (f fake) Changes(changes []metadata.Change) (original, proposed *types.Any, cas string, err error) {
	f <- 1
	return
}

func (f fake) Commit(proposed *types.Any, cas string) error {
	f <- 1
	return nil
}

func TestLazyNoBlockConnect(t *testing.T) {

	called := make(chan struct{})
	called2 := make(chan int, 2)

	g := UpdatableLazyConnect(func() (metadata.Updatable, error) {
		close(called)
		return fake(called2), nil
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.Get(types.PathFromString("a/b/c"))
		errs <- err
		close(errs)

		g.Commit(types.AnyString(""), "cas")
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
