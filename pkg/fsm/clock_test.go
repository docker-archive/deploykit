package fsm

import (
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/stretchr/testify/require"
)

func TestClock(t *testing.T) {
	clock := NewClock()

	done := make(chan struct{})
	start := make(chan struct{})
	go func() {
		defer close(done)
		<-start

		for {
			_, open := <-clock.C
			if !open {
				return // we expect this to be run
			}
		}
	}()

	log.Infoln("Start")
	close(start)
	clock.Start()
	clock.Tick()
	clock.Tick()
	clock.Tick()

	log.Infoln("Stopping")
	clock.Stop()

	log.Infoln("waiting for done")
	<-done
	log.Infoln("done")
}

func TestWallClock(t *testing.T) {

	ticker := time.After(100 * time.Millisecond)
	clock := Wall(ticker)

	start := make(chan struct{})
	go func() {
		<-start

		<-clock.C

		clock.Stop()
	}()

	close(start) // from here receive just 1 tick
	clock.Start()
	<-clock.C
}

func TestWallClock2(t *testing.T) {

	ticker := time.Tick(100 * time.Millisecond)
	clock := Wall(ticker)

	start := make(chan struct{})

	ticks := make(chan int, 1000)
	go func() {

		defer close(ticks)

		<-start

		for {
			_, open := <-clock.C
			if !open {
				return
			}
			log.Infoln("tick")
			ticks <- 1
		}
	}()

	close(start)
	clock.Start()
	log.Infoln("starting")

	time.Sleep(1 * time.Second)

	log.Infoln("Stopping")
	clock.Stop()
	log.Infoln("Stopped")

	total := 0
	for i := range ticks {
		total += i
	}
	log.Infoln("count=", total)
	require.Equal(t, 10, total)
}
