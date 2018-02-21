package internal

import (
	"sync"
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
	cleanup   func()
	running   bool
	lock      sync.Mutex
}

// Poll creates a poller
func Poll(shouldRun func() bool, work func() error, ticker <-chan time.Time) *Poller {
	return &Poller{
		err:       make(chan error),
		ticker:    ticker,
		stop:      make(chan interface{}),
		shouldRun: shouldRun,
		work:      work,
	}
}

// PollWithCleanup creates a poller with a clean up function that is invoked after the poller stops terminally.
func PollWithCleanup(shouldRun func() bool, work func() error, ticker <-chan time.Time, cleanup func()) *Poller {
	p := Poll(shouldRun, work, ticker)
	p.cleanup = cleanup
	return p
}

// Err returns the errors encountered by the poller
func (p *Poller) Err() error {
	return <-p.err
}

// Stop stops the Poller
func (p *Poller) Stop() {
	p.lock.Lock()
	defer p.lock.Unlock()

	close(p.stop)
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p *Poller) Run(ctx context.Context) {
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
		p.err = make(chan error, 2)
	}

	defer func() {
		if p.cleanup != nil {
			p.cleanup()
		}
	}()
	for {

		if p.shouldRun() {
			err := p.work()
			if err != nil {
				log.Warn("Poller error", "err", err)
				select {
				case p.err <- err:
				}
			}
		}

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

	}
}
