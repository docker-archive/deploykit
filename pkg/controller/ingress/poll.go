package ingress

import (
	"time"

	"golang.org/x/net/context"
)

// RunWithContext performs an operation using the provided context.
type RunWithContext interface {
	// Run starts running (non-blocking)
	Run(ctx context.Context) error

	// Stop stops running
	Stop()
}

// poller is the entity that polls for desired routes
type poller struct {
	name     string
	hardSync bool
	routes   Routes
	options  Options
	stop     chan interface{}
}

// Stop stops the poller
func (p *poller) Stop() {
	if p.stop != nil {
		close(p.stop)
	}
}

// Run will start all the matchers and query the services at defined polling interval.  It blocks until stop is called.
func (p *poller) Run(ctx context.Context) error {

	if p.stop != nil {
		return nil // already running
	}

	p.stop = make(chan interface{})

	ticker := time.Tick(p.options.Interval)

	for {
		select {

		case <-p.stop:
			log.Info("Stopping poller")
			return nil

		case <-ctx.Done():
			return ctx.Err()

		case <-ticker:

		}

		routesByVhosts, err := p.routes.List()
		if err != nil {
			return err
		}

		// push the routes to the loadbalancers
		log.Debug("found routes", "vhostRoutes", routesByVhosts)

	}
}
