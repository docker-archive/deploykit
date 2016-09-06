package util

import (
	mock_updater "github.com/docker/libmachete/mock/plugin/group/updater"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestSequentialWorkPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mock_updater.NewMockExecutor(ctrl)

	triggers := uint32(5)
	pool, err := NewWorkPool(triggers, executor, 1)
	require.NoError(t, err)

	executor.EXPECT().Proceed().Times(int(triggers))

	pool.Run()
}

func TestParallelWorkpool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mock_updater.NewMockExecutor(ctrl)

	pool, err := NewWorkPool(5, executor, 3)
	require.NoError(t, err)

	// Simulate the first item being slow to execute, proving that other items proceed.
	finished := make(chan bool)
	executor.EXPECT().Proceed().Do(func() {
		<-finished
	})

	gomock.InOrder(
		executor.EXPECT().Proceed().Times(3),
		executor.EXPECT().Proceed().Do(func() {
			finished <- true
		}),
	)

	pool.Run()
}

func TestWorkpoolStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mock_updater.NewMockExecutor(ctrl)

	pool, err := NewWorkPool(4, executor, 2)
	require.NoError(t, err)

	running := make(chan bool)
	stopped := make(chan bool)

	gomock.InOrder(
		executor.EXPECT().Proceed(),

		executor.EXPECT().Proceed().Do(func() {
			running <- true
			<-stopped
		}),

		executor.EXPECT().Proceed().Do(func() {
			<-stopped
		}).AnyTimes(),
	)

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go func() {
		pool.Run()
		waitGroup.Done()
	}()

	<-running
	pool.Stop()
	stopped <- true
	waitGroup.Wait()
}
