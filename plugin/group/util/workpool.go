package util

import (
	"github.com/docker/libmachete/controller/util"
	"sync"
)

// Executor is responsible for executing work for the pool.
type Executor interface {
	// Proceed runs one unit of work, blocking until it is complete.
	Proceed()
}

type workpool struct {
	triggers    uint32
	executor    Executor
	parallelism uint
	stop        chan bool
}

// NewWorkPool creates a work pool that will instruct an executor to perform work on items individually.  Work may
// be executed simultaneously, based on the requested parallelism.
func NewWorkPool(triggers uint32, executor Executor, parallelism uint) (util.RunStop, error) {
	return &workpool{
		triggers:    triggers,
		executor:    executor,
		parallelism: parallelism,
		stop:        make(chan bool),
	}, nil
}

func (u workpool) worker(triggers <-chan bool) {
	for {
		select {
		case _, ok := <-triggers:
			if !ok {
				return
			}

			u.executor.Proceed()
		case <-u.stop:
			return
		}
	}
}

func (u workpool) Run() {
	if u.parallelism <= 0 {
		panic("Parallelism must be greater than zero")
	}

	triggers := make(chan bool)
	var waitGroup sync.WaitGroup
	for i := uint(0); i < u.parallelism; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			u.worker(triggers)
		}()
	}

	for i := uint32(0); i < u.triggers; i++ {
		select {
		case triggers <- true:
			break
		case <-u.stop:
			return
		}
	}
	close(triggers)

	// Wait for in-flight work to finish.
	waitGroup.Wait()

	return
}

func (u workpool) Stop() {
	u.stop <- true
	close(u.stop)
}
