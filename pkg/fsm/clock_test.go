package fsm

import (
	"testing"
	"time"

	log "github.com/golang/glog"
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
	clock.Tick()
	clock.Tick()
	clock.Tick()
	clock.Stop()

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
	<-clock.C
}
