package ingress

import (
	"time"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"golang.org/x/net/context"
)

// Poller is the entity that polls for desired routes
type Poller struct {
	name          string
	hardSync      bool
	routes        func() (map[Vhost][]loadbalancer.Route, error)
	options       Options
	loadbalancers func() (map[Vhost]loadbalancer.L4, error)
	stop          chan interface{}
}

// Stop stops the Poller
func (p *Poller) Stop() {
	if p.stop != nil {
		close(p.stop)
	}
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p *Poller) Run(ctx context.Context) error {

	if p.stop != nil {
		return nil // already running
	}

	p.stop = make(chan interface{})

	ticker := time.Tick(p.options.Interval)

	for {
		select {

		case <-p.stop:
			log.Info("Stopping Poller")
			return nil

		case <-ctx.Done():
			return ctx.Err()

		case <-ticker:

		}

		log.Debug("running expose L4")
		err := ExposeL4(p.loadbalancers, p.routes, p.options)
		if err != nil {
			log.Warn("error exposing L4 routes", "err", err)
		}
	}
}
