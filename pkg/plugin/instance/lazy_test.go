package instance

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestLazyBlockAndCancel(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (instance.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 100*time.Second)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeInstances(nil, true)
		errs <- err
		close(errs)
	}()

	<-called

	CancelWait(g)

	require.Equal(t, "cancelled", (<-errs).Error())
}

func TestLazyNoBlock(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (instance.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeInstances(nil, true)
		errs <- err
		close(errs)
	}()

	<-called

	require.Equal(t, "boom", (<-errs).Error())
}

type fake chan int

func (f fake) Validate(req *types.Any) (err error) {
	f <- 1
	return
}

func (f fake) Provision(spec instance.Spec) (id *instance.ID, err error) {
	f <- 1
	return
}

func (f fake) Label(id instance.ID, labels map[string]string) (err error) {
	f <- 1
	return
}

func (f fake) Destroy(id instance.ID, context instance.Context) (err error) {
	f <- 1
	return
}

func (f fake) DescribeInstances(labels map[string]string, properties bool) (objects []instance.Description, err error) {
	f <- 1
	return
}

func TestLazyNoBlockConnect(t *testing.T) {

	called := make(chan struct{})
	called2 := make(chan int, 2)

	g := LazyConnect(func() (instance.Plugin, error) {
		close(called)
		return fake(called2), nil
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeInstances(nil, true)
		errs <- err
		close(errs)

		g.Destroy(instance.ID("test"), instance.Termination)
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
