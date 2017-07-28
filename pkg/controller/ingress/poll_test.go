package ingress

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestPollerShouldRun(t *testing.T) {

	shouldRun := make(chan bool, 1)
	work := make(chan error, 1)

	calledShouldRun := make(chan struct{})
	calledWork := make(chan struct{})

	poller := NewPoller(
		func() bool {
			close(calledShouldRun)
			return <-shouldRun
		},
		func() error {
			close(calledWork)
			return <-work
		},
		1*time.Second,
	)

	// replace the tick so we can control it
	tick := make(chan time.Time)
	poller.ticker = tick

	go poller.Run(context.Background())

	shouldRun <- true
	work <- nil

	tick <- time.Now()

	poller.Stop()

	<-calledShouldRun
	<-calledWork
}

func TestPollerShouldNotRun(t *testing.T) {

	shouldRun := make(chan bool, 1)
	work := make(chan error, 1)

	calledShouldRun := make(chan struct{})

	poller := NewPoller(
		func() bool {
			close(calledShouldRun)
			return <-shouldRun
		},
		func() error {
			panic("shouldn't call")
			return <-work
		},
		1*time.Second,
	)

	// replace the tick so we can control it
	tick := make(chan time.Time)
	poller.ticker = tick

	go poller.Run(context.Background())

	shouldRun <- false

	tick <- time.Now()

	poller.Stop()

	<-calledShouldRun
}

func TestPollerShouldRunError(t *testing.T) {

	shouldRun := make(chan bool, 1)
	work := make(chan error, 1)

	calledWork := make(chan struct{})

	poller := NewPoller(
		func() bool {
			return <-shouldRun
		},
		func() error {
			close(calledWork)
			return <-work
		},
		1*time.Second,
	)

	// replace the tick so we can control it
	tick := make(chan time.Time)
	poller.ticker = tick

	go poller.Run(context.Background())

	err := fmt.Errorf("test")
	shouldRun <- true
	work <- err

	tick <- time.Now()

	poller.Stop()

	require.Equal(t, err, <-poller.Err())
}
