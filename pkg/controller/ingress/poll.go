package ingress

import (
	"time"

	"golang.org/x/net/context"
)

// Poller is the entity that executes a unit of work at a predefined interval
type Poller struct {
	err       chan error
	ticker    <-chan time.Time
	stop      chan interface{}
	shouldRun func() bool
	work      func() error
	running   bool
}

// NewPoller creates a poller
func NewPoller(shouldRun func() bool, work func() error, interval time.Duration) *Poller {
	return &Poller{
		err:       make(chan error),
		ticker:    time.Tick(interval),
		stop:      make(chan interface{}),
		shouldRun: shouldRun,
		work:      work,
	}
}

// Err returns the errors encountered by the poller
func (p *Poller) Err() <-chan error {
	return p.err
}

// Stop stops the Poller
func (p Poller) Stop() {
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p Poller) Run(ctx context.Context) {
	if p.ticker == nil {
		panic("no ticker") // programming error.  not runtime.
	}

	if p.running {
		return
	}

	p.running = true
	if p.stop == nil {
		p.stop = make(chan interface{})
	}

	if p.err == nil {
		p.err = make(chan error)
	}

	for {
		select {

		case <-p.stop:
			log.Info("Stopping Poller")
			return

		case <-ctx.Done():
			select {
			case p.err <- ctx.Err():
			}

		case <-p.ticker:

		}

		if p.shouldRun() {
			err := p.work()
			if err != nil {
				log.Warn("Poller error", "err", err)
				select {
				case p.err <- err:
				}
			}
		}
	}
}
