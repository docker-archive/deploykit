package fsm

import (
	"time"
)

// Clock adapts a timer tick
type Clock struct {
	C    <-chan Tick
	c    chan<- Tick
	stop chan<- struct{}
}

// NewClock returns a clock
func NewClock() *Clock {
	c := make(chan Tick)
	stop := make(chan struct{})
	return &Clock{
		C:    c,
		c:    c,
		stop: stop,
	}
}

// Tick makes one tick of the clock
func (t *Clock) Tick() {
	t.c <- Tick(1)
}

// Stop stops the ticks
func (t *Clock) Stop() {
	if t.stop != nil {
		close(t.stop)
		close(t.c)
	}
}

// Wall adapts a regular time.Tick to return a clock
func Wall(tick <-chan time.Time) *Clock {
	out := make(chan Tick)
	stopper := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopper:
				return
			case <-tick:
				// note that golang's time ticker won't close the channel when stopped.
				// so we will do the closing ourselves to avoid leaking the goroutine
				out <- Tick(1)
			}
		}
	}()
	return &Clock{C: out, c: out, stop: stopper}
}
