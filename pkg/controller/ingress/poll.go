package ingress

import (
	"time"

	"golang.org/x/net/context"
)

// Poller is the entity that executes a unit of work at a predefined interval
type Poller struct {
	interval  time.Duration
	stop      chan interface{}
	shouldRun func() bool
	work      func() error
}

// Stop stops the Poller
func (p Poller) Stop() {
	if p.stop != nil {
		close(p.stop)
	}
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p Poller) Run(ctx context.Context) error {

	if p.stop != nil {
		return nil // already running
	}

	p.stop = make(chan interface{})

	ticker := time.Tick(p.interval)

	for {
		select {

		case <-p.stop:
			log.Info("Stopping Poller")
			return nil

		case <-ctx.Done():
			return ctx.Err()

		case <-ticker:

		}

		if p.shouldRun() {
			err := p.work()
			if err != nil {
				log.Warn("Poller error", "err", err)
			}
		}
	}
}
