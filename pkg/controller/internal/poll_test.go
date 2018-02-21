package internal

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

	calledShouldRun := make(chan int, 1)
	calledWork := make(chan int, 1)

	poller := Poll(
		func() bool {
			calledShouldRun <- 1
			return <-shouldRun
		},
		func() error {
			calledWork <- 1
			return <-work
		},
		time.Tick(1*time.Second),
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

	calledShouldRun := make(chan int, 1)

	poller := Poll(
		func() bool {
			calledShouldRun <- 1
			return <-shouldRun
		},
		func() error {
			panic("shouldn't call")
		},
		time.Tick(1*time.Second),
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

	poller := Poll(
		func() bool {
			return true
		},
		func() error {
			return fmt.Errorf("test")
		},
		time.Tick(1*time.Second),
	)

	// replace the tick so we can control it
	tick := make(chan time.Time)
	poller.ticker = tick

	go poller.Run(context.Background())

	require.Equal(t, fmt.Errorf("test"), poller.Err())

	poller.Stop()

}
