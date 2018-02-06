package fsm

import (
	"time"
)

// Clock adapts a timer tick
type Clock struct {
	C       <-chan Tick
	c       chan<- Tick
	stop    chan struct{}
	start   chan struct{}
	driver  func()
	running bool
}

// NewClock returns a clock
func NewClock() *Clock {
	c := make(chan Tick)
	stop := make(chan struct{})
	clock := &Clock{
		C:     c,
		c:     c,
		stop:  stop,
		start: make(chan struct{}),
	}
	clock.driver = func() {
		<-clock.start
		clock.start = nil

		for {
			select {
			case <-clock.stop:
				close(clock.c)
				return
			}
		}
	}
	return clock.run()
}

// Start starts the clock
func (t *Clock) Start() {
	if t.start != nil {
		close(t.start)
	}
}

// Tick makes one tick of the clock
func (t *Clock) Tick() {
	t.c <- Tick(1)
}

// Ticks makes multiple ticks
func (t *Clock) Ticks(ticks int) {
	for i := 0; i < ticks; i++ {
		t.Tick()
	}
}

func (t *Clock) run() *Clock {
	if t.driver != nil {
		go t.driver()
	}
	t.running = true
	return t
}

// Stop stops the ticks
func (t *Clock) Stop() {
	if t.running {
		close(t.stop)
		t.running = false
	}
}

// Wall adapts a regular time.Tick to return a clock
func Wall(tick <-chan time.Time) *Clock {
	out := make(chan Tick)
	stop := make(chan struct{})
	clock := &Clock{
		C:     out,
		c:     out,
		stop:  stop,
		start: make(chan struct{}),
	}

	clock.driver = func() {
		<-clock.start
		clock.start = nil

		for {
			select {
			case <-clock.stop:
				close(clock.c)
				return
			case <-tick:
				// note that golang's time ticker won't close the channel when stopped.
				// so we will do the closing ourselves to avoid leaking the goroutine
				clock.c <- Tick(1)
			}
		}
	}

	return clock.run()
}
