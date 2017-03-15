package fsm

import (
	"sync"
	"time"
)

// Clock adapts a timer tick
type Clock struct {
	C       <-chan Tick
	c       chan<- Tick
	stop    chan<- struct{}
	driver  func()
	running bool
	lock    sync.Mutex
}

// NewClock returns a clock
func NewClock() *Clock {
	c := make(chan Tick)
	stop := make(chan struct{})
	clock := &Clock{
		C:    c,
		c:    c,
		stop: stop,
	}
	return clock.run()
}

// Tick makes one tick of the clock
func (t *Clock) Tick() {
	t.c <- Tick(1)
}

// Stop stops the ticks
func (t *Clock) Stop() {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.running {
		close(t.stop)
		close(t.c)
		t.running = false
	}
}

func (t *Clock) run() *Clock {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.driver != nil {
		go t.driver()
	}
	t.running = true
	return t
}

// Wall adapts a regular time.Tick to return a clock
func Wall(tick <-chan time.Time) *Clock {
	out := make(chan Tick)
	stop := make(chan struct{})
	clock := &Clock{
		C:    out,
		c:    out,
		stop: stop,
		driver: func() {
			for {
				select {
				case <-stop:
					return
				case <-tick:
					// note that golang's time ticker won't close the channel when stopped.
					// so we will do the closing ourselves to avoid leaking the goroutine
					out <- Tick(1)
				}
			}
		},
	}
	return clock.run()
}
